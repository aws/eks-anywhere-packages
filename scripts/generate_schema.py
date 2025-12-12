import yaml
import json
import sys
import argparse
from typing import Dict, Any, Union, List, Optional

# Global variables for old schema and interactive mode
old_schema_data = None
interactive_mode = False
interactive_responses = {}

def get_from_path(data: Dict, path: str) -> Optional[Any]:
    """Navigate nested dict using dot-separated path."""
    keys = path.split('.')
    current = data
    for key in keys:
        if isinstance(current, dict) and key in current:
            current = current[key]
        else:
            return None
    return current

def guess_type_from_name(key: str) -> str:
    """Guess type based on field name patterns."""
    key_lower = key.lower()
    
    # Integer patterns
    if key_lower.endswith('port') or key_lower.endswith('seconds') or 'timeout' in key_lower:
        return "integer"
    
    # Array patterns  
    if key_lower.endswith('ranges') or key_lower.endswith('ips') or key_lower.endswith('list'):
        return "array"
    
    # Default to string for most cases
    return "string"

def prompt_for_type(field_path: str, key: str) -> str:
    """Interactively ask user for type of a field."""
    # Check if we already asked about this field
    if field_path in interactive_responses:
        return interactive_responses[field_path]
    
    guessed = guess_type_from_name(key)
    print(f"\n🔍 Field '{field_path}' has null/empty value.")
    print(f"   Best guess: {guessed}")
    print("   Choose type:")
    print("   1) string")
    print("   2) integer")
    print("   3) array")
    print("   4) object")
    print("   5) boolean")
    print(f"   6) keep null")
    print(f"   Press ENTER to use guess ({guessed})")
    
    choice = input("   > ").strip()
    
    type_map = {
        "1": "string",
        "2": "integer", 
        "3": "array",
        "4": "object",
        "5": "boolean",
        "6": "null",
        "": guessed  # Use guess if Enter pressed
    }
    
    chosen_type = type_map.get(choice, guessed)
    interactive_responses[field_path] = chosen_type
    return chosen_type

def generate_schema_for_value(value: Any, key: str = "", path: str = "") -> Dict[str, Any]:
    """Generate JSON schema for a given YAML value."""
    schema = {}
    current_path = f"{path}.{key}" if path else key

    if isinstance(value, dict):
        schema["type"] = "object"
        schema["default"] = {}
        schema["title"] = f"The {key} Schema" if key else "Root Schema"
        
        # Try to preserve 'required' from old schema
        schema["required"] = []
        if old_schema_data and current_path:
            old_field = get_from_path(old_schema_data, f"properties.{current_path}".replace("..", "."))
            if old_field and isinstance(old_field, dict) and "required" in old_field:
                schema["required"] = old_field["required"]
                print(f"ℹ️  Preserving required fields from old schema for '{current_path}': {old_field['required']}")
        
        schema["additionalProperties"] = True
        schema["properties"] = {}

        # Process properties
        for k, v in value.items():
            schema["properties"][k] = generate_schema_for_value(v, k, current_path)

        if value:
            schema["examples"] = [value]

    elif isinstance(value, list):
        schema["type"] = "array"
        schema["default"] = []
        schema["title"] = f"The {key} Schema" if key else "The Schema"

        if value:
            item_schema = generate_schema_for_value(value[0], "", current_path)
            
            # Try to preserve required fields for array item schemas
            if item_schema.get("type") == "object" and old_schema_data and current_path:
                # Build proper path with .properties. between levels
                path_parts = current_path.split('.')
                old_path = "properties." + ".properties.".join(path_parts)
                old_array = get_from_path(old_schema_data, old_path)
                if old_array and isinstance(old_array, dict) and "items" in old_array:
                    old_items = old_array["items"]
                    if isinstance(old_items, dict) and "required" in old_items and old_items["required"]:
                        item_schema["required"] = old_items["required"]
                        print(f"ℹ️  Preserving required fields for array items in '{current_path}': {old_items['required']}")
            
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
        # For null or other types - this is where we enhance!
        if value is None and old_schema_data:
            # Try to get type from old schema
            old_field = get_from_path(old_schema_data, f"properties.{current_path}".replace("..", "."))
            if old_field and isinstance(old_field, dict):
                # Preserve the old schema definition for this field
                print(f"ℹ️  Preserving type from old schema for '{current_path}'")
                return old_field
        
        # If we can't find in old schema, determine type
        if value is None:
            if interactive_mode:
                actual_type = prompt_for_type(current_path, key)
            else:
                actual_type = guess_type_from_name(key)
            
            if actual_type == "null":
                schema["type"] = "null"
            else:
                # Create anyOf pattern for optional fields
                schema["anyOf"] = [
                    {"type": actual_type},
                    {"type": "null"}
                ]
        else:
            schema["type"] = "string"
        
        # Preserve null as default for null values, regardless of anyOf types
        if value is None:
            schema["default"] = None
        else:
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

    schema.update(generate_schema_for_value(yaml_data))

    return schema

def main():
    parser = argparse.ArgumentParser(
        description='Generate JSON schema from Helm values.yaml file',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog='''
Examples:
  # Basic usage
  python3 generate_schema.py values.yaml schema.json
  
  # Reference old schema to preserve types
  python3 generate_schema.py values.yaml schema.json --old-schema old-schema.json
  
  # Interactive mode for unknown fields
  python3 generate_schema.py values.yaml schema.json --old-schema old-schema.json --interactive
        '''
    )
    
    parser.add_argument('yaml_file', help='Input values.yaml file')
    parser.add_argument('output_file', nargs='?', help='Output schema.json file (optional, prints to stdout if not provided)')
    parser.add_argument('--old-schema', help='Reference old schema.json to preserve type definitions')
    parser.add_argument('--interactive', action='store_true', help='Prompt for types of unknown null-valued fields')
    
    args = parser.parse_args()

    global old_schema_data, interactive_mode
    interactive_mode = args.interactive

    try:
        # Load old schema if provided
        if args.old_schema:
            print(f"📖 Loading old schema from {args.old_schema}")
            with open(args.old_schema, 'r') as f:
                old_schema_data = json.load(f)
        
        # Load values.yaml
        with open(args.yaml_file, 'r') as f:
            yaml_data = yaml.safe_load(f)

        schema = generate_json_schema(yaml_data)

        # Output the schema
        if args.output_file:
            with open(args.output_file, 'w') as f:
                json.dump(schema, f, indent=4)
            print(f"✅ Schema written to {args.output_file}")
            
            if interactive_mode and interactive_responses:
                print(f"\n📝 Made {len(interactive_responses)} interactive type decisions")
        else:
            print(json.dumps(schema, indent=4))

    except Exception as e:
        print(f"❌ Error: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc()
        sys.exit(1)

if __name__ == "__main__":
    main()
