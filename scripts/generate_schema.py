import yaml
import json
import sys
from typing import Dict, Any, Union, List

def generate_schema_for_value(value: Any, key: str = "") -> Dict[str, Any]:
    """Generate JSON schema for a given YAML value."""
    schema = {}

    if isinstance(value, dict):
        schema["type"] = "object"
        schema["default"] = {}
        schema["title"] = f"The {key} Schema" if key else "Root Schema"
        schema["required"] = []  # Empty required field by default
        schema["additionalProperties"] = True
        schema["properties"] = {}

        # Process properties
        for k, v in value.items():
            schema["properties"][k] = generate_schema_for_value(v, k)

        # Add example if it's not an empty dict
        if value:
            schema["examples"] = [value]

    elif isinstance(value, list):
        schema["type"] = "array"
        schema["default"] = []
        schema["title"] = f"The {key} Schema" if key else "The Schema"

        # Determine item type if list is not empty
        if value:
            # For simplicity, assume all items are of the same type
            item_schema = generate_schema_for_value(value[0])
            schema["items"] = item_schema if item_schema["type"] == "object" else {"type": item_schema["type"]}

        schema["examples"] = [value] if value else []

    elif isinstance(value, bool):
        schema["type"] = "boolean"
        schema["default"] = value
        schema["title"] = f"The {key} Schema" if key else "The Schema"
        schema["examples"] = [value]

    elif isinstance(value, int):
        schema["type"] = "integer"
        schema["default"] = value
        schema["title"] = f"The {key} Schema" if key else "The Schema"
        schema["examples"] = [value]

    elif isinstance(value, float):
        schema["type"] = "number"
        schema["default"] = value
        schema["title"] = f"The {key} Schema" if key else "The Schema"
        schema["examples"] = [value]

    elif isinstance(value, str):
        schema["type"] = "string"
        schema["default"] = value
        schema["title"] = f"The {key} Schema" if key else "The Schema"
        schema["examples"] = [value]

    else:
        # For null or other types
        schema["type"] = "null" if value is None else "string"
        schema["default"] = value
        schema["title"] = f"The {key} Schema" if key else "The Schema"
        schema["examples"] = [value] if value is not None else []

    return schema

def generate_json_schema(yaml_data: Dict[str, Any]) -> Dict[str, Any]:
    """Generate a complete JSON schema from YAML data."""
    schema = {
        "$schema": "https://json-schema.org/draft/2019-09/schema",
        "$id": "http://example.com/example.json",
    }

    # Merge with the generated schema for the YAML data
    schema.update(generate_schema_for_value(yaml_data))

    return schema

def main():
    if len(sys.argv) < 2:
        print("Usage: python script.py <yaml_file> [output_file]")
        sys.exit(1)

    yaml_file = sys.argv[1]

    # Check if output file is provided
    output_file = None
    if len(sys.argv) > 2:
        output_file = sys.argv[2]

    try:
        with open(yaml_file, 'r') as f:
            yaml_data = yaml.safe_load(f)

        schema = generate_json_schema(yaml_data)

        # Output the schema to file or stdout
        if output_file:
            with open(output_file, 'w') as f:
                json.dump(schema, f, indent=4)
            print(f"Schema written to {output_file}")
        else:
            print(json.dumps(schema, indent=4))

    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()