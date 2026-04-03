use std::collections::HashMap;

use crate::types::SpadeType;

/// A single input or output entry in a block manifest.
#[derive(Debug, Clone, PartialEq)]
pub struct ManifestEntry {
    pub type_name: String,
    pub format: Option<String>,
    pub item_type: Option<String>,
}

impl ManifestEntry {
    fn to_yaml_value(&self) -> serde_yaml::Value {
        let mut map = serde_yaml::Mapping::new();
        map.insert(
            serde_yaml::Value::String("type".into()),
            serde_yaml::Value::String(self.type_name.clone()),
        );
        if let Some(ref fmt) = self.format {
            map.insert(
                serde_yaml::Value::String("format".into()),
                serde_yaml::Value::String(fmt.clone()),
            );
        }
        if let Some(ref it) = self.item_type {
            map.insert(
                serde_yaml::Value::String("item_type".into()),
                serde_yaml::Value::String(it.clone()),
            );
        }
        serde_yaml::Value::Mapping(map)
    }
}

/// Fluent builder for generating block manifest dictionaries.
///
/// # Example
///
/// ```
/// use spade::{ManifestBuilder, RasterFile};
///
/// let manifest = ManifestBuilder::new()
///     .description("Reprojects a raster")
///     .input::<RasterFile>("source")
///     .input::<f64>("resolution")
///     .output::<RasterFile>("raster")
///     .build();
///
/// assert!(manifest.contains_key("inputs"));
/// assert!(manifest.contains_key("outputs"));
/// ```
pub struct ManifestBuilder {
    description: Option<String>,
    inputs: Vec<(String, ManifestEntry)>,
    outputs: Vec<(String, ManifestEntry)>,
}

impl ManifestBuilder {
    /// Create a new empty manifest builder.
    pub fn new() -> Self {
        Self {
            description: None,
            inputs: Vec::new(),
            outputs: Vec::new(),
        }
    }

    /// Set the block description.
    pub fn description(mut self, desc: impl Into<String>) -> Self {
        self.description = Some(desc.into());
        self
    }

    /// Declare an input with its Spade type.
    pub fn input<T: SpadeType>(mut self, name: impl Into<String>) -> Self {
        let info = T::manifest_entry();
        self.inputs.push((
            name.into(),
            ManifestEntry {
                type_name: info.type_name.to_string(),
                format: info.format.map(|s| s.to_string()),
                item_type: info.item_type.map(|s| s.to_string()),
            },
        ));
        self
    }

    /// Declare an output with its Spade type.
    pub fn output<T: SpadeType>(mut self, name: impl Into<String>) -> Self {
        let info = T::manifest_entry();
        self.outputs.push((
            name.into(),
            ManifestEntry {
                type_name: info.type_name.to_string(),
                format: info.format.map(|s| s.to_string()),
                item_type: info.item_type.map(|s| s.to_string()),
            },
        ));
        self
    }

    /// Build the manifest as a YAML-compatible map.
    pub fn build(self) -> HashMap<String, serde_yaml::Value> {
        let mut manifest = HashMap::new();

        if let Some(desc) = self.description {
            manifest.insert(
                "description".to_string(),
                serde_yaml::Value::String(desc),
            );
        }

        // Inputs
        let mut inputs_map = serde_yaml::Mapping::new();
        for (name, entry) in &self.inputs {
            inputs_map.insert(
                serde_yaml::Value::String(name.clone()),
                entry.to_yaml_value(),
            );
        }
        manifest.insert(
            "inputs".to_string(),
            serde_yaml::Value::Mapping(inputs_map),
        );

        // Outputs
        let mut outputs_map = serde_yaml::Mapping::new();
        for (name, entry) in &self.outputs {
            outputs_map.insert(
                serde_yaml::Value::String(name.clone()),
                entry.to_yaml_value(),
            );
        }
        manifest.insert(
            "outputs".to_string(),
            serde_yaml::Value::Mapping(outputs_map),
        );

        manifest
    }
}

impl Default for ManifestBuilder {
    fn default() -> Self {
        Self::new()
    }
}

/// Create a new `ManifestBuilder`.
pub fn build() -> ManifestBuilder {
    ManifestBuilder::new()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{
        Directory, File, FileCollection, JsonFile, RasterFile, RasterFileCollection, VectorFile,
    };

    fn get_input_field(
        manifest: &HashMap<String, serde_yaml::Value>,
        input_name: &str,
        field: &str,
    ) -> Option<String> {
        manifest
            .get("inputs")
            .and_then(|v| v.as_mapping())
            .and_then(|m| m.get(serde_yaml::Value::String(input_name.into())))
            .and_then(|v| v.as_mapping())
            .and_then(|m| m.get(serde_yaml::Value::String(field.into())))
            .and_then(|v| v.as_str())
            .map(|s| s.to_string())
    }

    fn get_output_field(
        manifest: &HashMap<String, serde_yaml::Value>,
        output_name: &str,
        field: &str,
    ) -> Option<String> {
        manifest
            .get("outputs")
            .and_then(|v| v.as_mapping())
            .and_then(|m| m.get(serde_yaml::Value::String(output_name.into())))
            .and_then(|v| v.as_mapping())
            .and_then(|m| m.get(serde_yaml::Value::String(field.into())))
            .and_then(|v| v.as_str())
            .map(|s| s.to_string())
    }

    #[test]
    fn test_simple_file_input() {
        let manifest = ManifestBuilder::new()
            .input::<File>("source")
            .build();

        assert_eq!(get_input_field(&manifest, "source", "type"), Some("file".into()));
    }

    #[test]
    fn test_typed_file_inputs() {
        let manifest = ManifestBuilder::new()
            .input::<RasterFile>("raster")
            .input::<VectorFile>("vector")
            .build();

        assert_eq!(get_input_field(&manifest, "raster", "type"), Some("file".into()));
        assert_eq!(
            get_input_field(&manifest, "raster", "format"),
            Some("GeoTIFF".into())
        );
        assert_eq!(get_input_field(&manifest, "vector", "type"), Some("file".into()));
        assert_eq!(
            get_input_field(&manifest, "vector", "format"),
            Some("GeoJSON".into())
        );
    }

    #[test]
    fn test_scalar_inputs() {
        let manifest = ManifestBuilder::new()
            .input::<String>("name")
            .input::<f64>("count")
            .input::<bool>("enabled")
            .build();

        assert_eq!(get_input_field(&manifest, "name", "type"), Some("string".into()));
        assert_eq!(get_input_field(&manifest, "count", "type"), Some("number".into()));
        assert_eq!(
            get_input_field(&manifest, "enabled", "type"),
            Some("boolean".into())
        );
    }

    #[test]
    fn test_collection_input() {
        let manifest = ManifestBuilder::new()
            .input::<RasterFileCollection>("tiles")
            .build();

        assert_eq!(
            get_input_field(&manifest, "tiles", "type"),
            Some("collection".into())
        );
        assert_eq!(
            get_input_field(&manifest, "tiles", "item_type"),
            Some("file".into())
        );
        assert_eq!(
            get_input_field(&manifest, "tiles", "format"),
            Some("GeoTIFF".into())
        );
    }

    #[test]
    fn test_output_declarations() {
        let manifest = ManifestBuilder::new()
            .input::<File>("source")
            .output::<RasterFile>("raster")
            .build();

        assert_eq!(
            get_output_field(&manifest, "raster", "type"),
            Some("file".into())
        );
        assert_eq!(
            get_output_field(&manifest, "raster", "format"),
            Some("GeoTIFF".into())
        );
    }

    #[test]
    fn test_description() {
        let manifest = ManifestBuilder::new()
            .description("Processes input data.")
            .input::<File>("source")
            .build();

        assert_eq!(
            manifest.get("description"),
            Some(&serde_yaml::Value::String("Processes input data.".into()))
        );
    }

    #[test]
    fn test_no_description() {
        let manifest = ManifestBuilder::new()
            .input::<File>("source")
            .build();

        assert!(!manifest.contains_key("description"));
    }

    #[test]
    fn test_mixed_inputs() {
        let manifest = ManifestBuilder::new()
            .description("Normalizes raster data.")
            .input::<RasterFile>("raster")
            .input::<i64>("buffer")
            .input::<bool>("normalize")
            .output::<RasterFile>("raster")
            .build();

        assert_eq!(get_input_field(&manifest, "raster", "type"), Some("file".into()));
        assert_eq!(
            get_input_field(&manifest, "raster", "format"),
            Some("GeoTIFF".into())
        );
        assert_eq!(get_input_field(&manifest, "buffer", "type"), Some("number".into()));
        assert_eq!(
            get_input_field(&manifest, "normalize", "type"),
            Some("boolean".into())
        );
        assert_eq!(
            get_output_field(&manifest, "raster", "type"),
            Some("file".into())
        );
        assert_eq!(
            manifest.get("description"),
            Some(&serde_yaml::Value::String("Normalizes raster data.".into()))
        );
    }

    #[test]
    fn test_directory_input() {
        let manifest = ManifestBuilder::new()
            .input::<Directory>("source")
            .build();

        assert_eq!(
            get_input_field(&manifest, "source", "type"),
            Some("directory".into())
        );
    }

    #[test]
    fn test_json_input() {
        let manifest = ManifestBuilder::new()
            .input::<JsonFile>("config")
            .build();

        assert_eq!(get_input_field(&manifest, "config", "type"), Some("json".into()));
    }

    #[test]
    fn test_file_collection_input() {
        let manifest = ManifestBuilder::new()
            .input::<FileCollection>("data")
            .build();

        assert_eq!(
            get_input_field(&manifest, "data", "type"),
            Some("collection".into())
        );
        assert_eq!(
            get_input_field(&manifest, "data", "item_type"),
            Some("file".into())
        );
    }

    #[test]
    fn test_build_convenience_function() {
        let manifest = build()
            .input::<File>("source")
            .output::<File>("result")
            .build();

        assert!(manifest.contains_key("inputs"));
        assert!(manifest.contains_key("outputs"));
    }
}
