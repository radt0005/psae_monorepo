//! Small parameter parsers shared by blocks.

/// Split a comma-separated string into trimmed, non-empty tokens.
pub fn parse_csv_list(s: &str) -> Vec<String> {
    s.split(',')
        .map(|t| t.trim().to_string())
        .filter(|t| !t.is_empty())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_trims_and_drops_empties() {
        assert_eq!(
            parse_csv_list(" a , b ,, c,"),
            vec!["a".to_string(), "b".into(), "c".into()]
        );
    }

    #[test]
    fn empty_string_empty_list() {
        assert!(parse_csv_list("").is_empty());
        assert!(parse_csv_list(" , , ").is_empty());
    }
}
