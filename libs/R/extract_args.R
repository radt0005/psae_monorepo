#' Convert function documentation to a JSON schema
#'
#' This function converts function documentation to an R list that represents
#' a JSON schema. The JSON schema can be used as input LLM providers
#' which support native function calling and require a JSON schema to
#' describe function and its arguments.
#'
#' @param docs Function documentation as returned by [tools_get_docs()]
#' @param all_required Logical indicating whether all arguments should be required.
#' 'TRUE' currently required for the OpenAI API
#' @param additional_properties Logical indicating whether additional properties
#' are allowed. See your LLM provider's documentation to know if this is
#' supported
#'
#' @return A list (R object) representing a JSON schema for the function
#'
#' @noRd
#' @keywords internal
tools_docs_to_r_json_schema <- function(
    docs,
    all_required = TRUE,
    additional_properties = FALSE
    ) {
    # Helper function to process each argument recursively
    process_argument <- function(arg) {
        prop <- list()
        r_type <- arg$type

        if (is.list(r_type)) {
        # Handle named lists (objects)
        prop$type <- "object"
        prop$additionalProperties <- FALSE # Set additionalProperties to FALSE
        prop$properties <- list()
        for (name in names(r_type)) {
            sub_arg <- list(
            type = r_type[[name]],
            default_value = if (!is.null(arg$default_value[[name]]))
                arg$default_value[[name]] else NULL
            )
            prop$properties[[name]] <- process_argument(sub_arg)
        }
        } else if (is.null(r_type) || r_type == "unknown") {
        prop$type <- "string"
        } else if (r_type == "character") {
        prop$type <- "string"
        } else if (r_type == "integer") {
        prop$type <- "integer"
        } else if (r_type == "numeric") {
        prop$type <- "number"
        } else if (r_type == "logical") {
        prop$type <- "boolean"
        } else if (r_type == "match.arg") {
        prop$type <- "string"
        # Check if default_value is a call, e.g. c("Val1", "Val2", ...)
        if (
            is.call(arg$default_value) &&
            identical(arg$default_value[[1]], as.name("c"))
        ) {
            # Evaluate the call to get a standard R vector
            prop$enum <- eval(arg$default_value)
        } else {
            prop$enum <- arg$default_value
        }
        } else if (grepl("^vector ", r_type)) {
        # Handle vector types
        item_type <- sub("^vector ", "", r_type)
        prop$type <- "array"
        if (item_type == "integer") {
            prop$items <- list(type = "integer")
        } else if (item_type == "numeric") {
            prop$items <- list(type = "number")
        } else if (item_type %in% c("string", "character")) {
            prop$items <- list(type = "string")
        } else if (item_type == "logical") {
            prop$items <- list(type = "boolean")
        } else {
            prop$items <- list()
        }
        } else if (r_type == "list") {
        # Handle unnamed lists
        default_value <- arg$default_value
        if (is.null(default_value)) {
            prop$type <- "array"
            prop$items <- list()
        } else if (is.list(default_value)) {
            if (is.null(names(default_value))) {
            # Unnamed list (array)
            prop$type <- "array"
            if (length(default_value) > 0) {
                # Assume homogeneous items
                sub_arg <- list(
                type = arg$type,
                default_value = default_value[[1]]
                )
                prop$items <- process_argument(sub_arg)
            } else {
                prop$items <- list()
            }
            } else {
            # Named list (object)
            prop$type <- "object"
            prop$additionalProperties <- FALSE # Set additionalProperties to FALSE
            prop$properties <- list()
            for (name in names(default_value)) {
                sub_arg <- list(
                type = arg$type[[name]],
                default_value = default_value[[name]]
                )
                prop$properties[[name]] <- process_argument(sub_arg)
            }
            }
        } else {
            prop$type <- "array"
            prop$items <- list()
        }
        } else if (r_type == "call") {
        prop$type <- "string"
        } else {
        prop$type <- "string"
        }
        return(prop)
    }

    # Process arguments
    args <- docs$arguments
    properties <- list()
    required_args <- character(0)

    for (arg_name in names(args)) {
        arg <- args[[arg_name]]
        prop <- process_argument(arg)

        # Add to required arguments if there's no default value
        if (!"default_value" %in% names(arg)) {
        required_args <- c(required_args, arg_name)
        }

        # Add property to properties list
        properties[[arg_name]] <- prop
    }

    if (all_required) required_args <- names(args)

    list(
        type = "object",
        properties = properties,
        required = required_args,
        additionalProperties = additional_properties
    )
}



#docs <- tidyprompt::tools_get_docs(name)
#schema <- tools_docs_to_r_json_schema(docs)
#json_schema <- rjson::toJSON(schema)
#write(json_schema, file = "{schema_path}")
