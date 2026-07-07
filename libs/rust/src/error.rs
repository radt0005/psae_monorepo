use thiserror::Error;

/// Errors that can occur during Spade block execution.
#[derive(Error, Debug)]
pub enum SpadeError {
    /// Wraps filesystem I/O errors.
    #[error("I/O error: {0}")]
    IoError(#[from] std::io::Error),

    /// Wraps YAML parsing errors.
    #[error("YAML error: {0}")]
    YamlError(#[from] serde_yaml::Error),

    /// Wraps JSON parsing errors.
    #[error("JSON error: {0}")]
    JsonError(#[from] serde_json::Error),

    /// A requested input name does not exist in the `inputs/` directory.
    #[error("input not found: '{name}'")]
    InputNotFound { name: String },

    /// A requested parameter does not exist in `params.yaml`.
    #[error("parameter not found: '{name}'")]
    ParamNotFound { name: String },

    /// A requested secret was not provided to the block.
    #[error("secret not found: '{name}'; declare it in the pipeline's secrets mapping")]
    SecretNotFound { name: String },

    /// An input subdirectory exists but contains no files.
    #[error("input directory '{name}' is empty")]
    EmptyInputDir { name: String },

    /// Input value cannot be converted to the requested type.
    #[error("type mismatch for '{name}': expected {expected}, found {found}")]
    TypeMismatch {
        name: String,
        expected: &'static str,
        found: &'static str,
    },

    /// Wraps errors from user handler functions.
    #[error("handler error: {0}")]
    HandlerError(Box<dyn std::error::Error + Send + Sync>),
}

/// Convenience type alias for Spade results.
pub type Result<T> = std::result::Result<T, SpadeError>;
