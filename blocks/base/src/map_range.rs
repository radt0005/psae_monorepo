//! Block: `base.map_range`
//!
//! Fan out over a numeric range `[start, end)` with a configurable step.

use std::path::{Path, PathBuf};

use spade::Args;

use crate::common::expansion;
use crate::common::{BaseError, Result};

const MAX_ITEMS: usize = 1_000_000;

pub(crate) fn handler(args: Args, base: &Path) -> Result<()> {
    let start: f64 = args.param("start")?;
    let end: f64 = args.param("end")?;
    let step: f64 = args.param::<f64>("step").unwrap_or(1.0);

    let values = generate_values(start, end, step)?;
    if values.is_empty() {
        return Err(BaseError::EmptyCollection);
    }

    let items = expansion::materialise_scalar_items(base, &values)?;
    expansion::write_manifest(base, items)?;
    Ok(())
}

fn generate_values(start: f64, end: f64, step: f64) -> Result<Vec<serde_json::Value>> {
    if !start.is_finite() || !end.is_finite() || !step.is_finite() {
        return Err(BaseError::BadExpression(
            "start, end, and step must be finite".into(),
        ));
    }
    if step == 0.0 {
        return Err(BaseError::BadExpression("step must be non-zero".into()));
    }
    if start == end {
        return Err(BaseError::BadExpression("start must not equal end".into()));
    }
    if (end - start).signum() != step.signum() {
        return Err(BaseError::BadExpression(
            "step sign must match direction from start to end".into(),
        ));
    }

    let integer_values = start.fract() == 0.0 && end.fract() == 0.0 && step.fract() == 0.0;

    let mut out = Vec::new();
    if integer_values {
        let mut v = start as i64;
        let e = end as i64;
        let s = step as i64;
        if s > 0 {
            while v < e {
                out.push(serde_json::json!(v));
                if out.len() > MAX_ITEMS {
                    return Err(BaseError::BadExpression(format!(
                        "range would emit more than {MAX_ITEMS} items"
                    )));
                }
                v += s;
            }
        } else {
            while v > e {
                out.push(serde_json::json!(v));
                if out.len() > MAX_ITEMS {
                    return Err(BaseError::BadExpression(format!(
                        "range would emit more than {MAX_ITEMS} items"
                    )));
                }
                v += s;
            }
        }
    } else {
        // f64 path. Iterate by multiplication from start to avoid drift.
        let span = (end - start) / step;
        let n = span.floor() as i64;
        if n > MAX_ITEMS as i64 {
            return Err(BaseError::BadExpression(format!(
                "range would emit more than {MAX_ITEMS} items"
            )));
        }
        let direction_positive = step > 0.0;
        for i in 0..=n {
            let v = start + (i as f64) * step;
            if direction_positive {
                if v >= end {
                    break;
                }
            } else if v <= end {
                break;
            }
            out.push(serde_json::json!(v));
        }
    }
    Ok(out)
}

pub fn entry() {
    let base = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    spade::run(move |args| handler(args, &base).map_err(Into::into));
}

pub fn run_in(base: &Path) -> Result<()> {
    let args = spade::scanning::build_args_from(base)?;
    handler(args, base)?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn integer_range_ascending() {
        let v = generate_values(0.0, 5.0, 1.0).unwrap();
        assert_eq!(v.len(), 5);
        assert_eq!(v[0], serde_json::json!(0));
        assert_eq!(v[4], serde_json::json!(4));
    }

    #[test]
    fn float_range_quarter_step() {
        let v = generate_values(0.0, 1.0, 0.25).unwrap();
        assert_eq!(v.len(), 4);
    }

    #[test]
    fn zero_step_rejected() {
        assert!(generate_values(0.0, 5.0, 0.0).is_err());
    }

    #[test]
    fn start_eq_end_rejected() {
        assert!(generate_values(3.0, 3.0, 1.0).is_err());
    }

    #[test]
    fn too_many_items_rejected() {
        assert!(generate_values(0.0, 2_000_001.0, 1.0).is_err());
    }

    #[test]
    fn wrong_step_sign_rejected() {
        assert!(generate_values(0.0, 5.0, -1.0).is_err());
    }
}
