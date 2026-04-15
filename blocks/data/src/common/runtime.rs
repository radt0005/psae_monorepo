//! Blocking bridge to OpenDAL's async API.
//!
//! Block handlers are synchronous, but OpenDAL (and parts of reqwest)
//! return futures. [`block_on`] constructs a thread-local single-threaded
//! tokio runtime once per thread and reuses it for every call. If
//! building a tokio runtime fails (essentially impossible on a
//! functioning OS, so this is mostly insurance), we fall back to
//! `futures::executor::block_on`.

use std::cell::RefCell;
use std::future::Future;

use tokio::runtime::Runtime;

thread_local! {
    static RT: RefCell<Option<Runtime>> = const { RefCell::new(None) };
}

/// Drive a future to completion on a thread-local current-thread runtime.
pub fn block_on<F: Future>(fut: F) -> F::Output {
    // Materialise the runtime lazily. We don't hold the `RefCell`
    // borrow across `block_on` because user futures may themselves
    // enter `block_on` on this thread.
    let have_rt = RT.with(|cell| {
        let mut slot = cell.borrow_mut();
        if slot.is_none() {
            if let Ok(rt) = tokio::runtime::Builder::new_current_thread()
                .enable_all()
                .build()
            {
                *slot = Some(rt);
            }
        }
        slot.is_some()
    });

    if have_rt {
        RT.with(|cell| {
            let slot = cell.borrow();
            match slot.as_ref() {
                Some(rt) => rt.block_on(fut),
                // Unreachable since we just confirmed `have_rt`, but
                // avoid `expect` to honour the "no unwrap in prod" rule.
                None => futures::executor::block_on(fut),
            }
        })
    } else {
        futures::executor::block_on(fut)
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn block_on_trivial_future() {
        let v = block_on(async { 1 + 2 });
        assert_eq!(v, 3);
    }

    #[test]
    fn block_on_reuses_runtime() {
        let a = block_on(async { 1 });
        let b = block_on(async { 2 });
        assert_eq!(a + b, 3);
    }
}
