def run(ctx):
    path = "../devlore-cli/internal/execution"
    note("Parsing execution schema from: " + path)
    schema = go.parse_execution_schema(path)

    note("All attributes:")
    for attr in dir(schema):
        if attr.startswith("_"):
            continue
        val = getattr(schema, attr)
        if hasattr(val, "name"):
            note("  " + attr + ": struct " + val.name + " with " + str(len(list(val.fields))) + " fields")
        elif hasattr(val, "__len__"):
            items = list(val)
            if len(items) > 0 and type(items[0]) == "string":
                note("  " + attr + ": [" + ", ".join(items[:5]) + ("..." if len(items) > 5 else "") + "]")
            else:
                note("  " + attr + ": " + str(len(items)) + " items")
        else:
            note("  " + attr + ": " + str(val))

    # Show Node fields
    if hasattr(schema, "node"):
        note("\nNode fields:")
        for f in schema.node.fields:
            note("  " + f.json_name + " (" + f.type + ") required=" + str(f.required))

    # Show operations
    if hasattr(schema, "operations"):
        note("\nOperations: " + ", ".join(list(schema.operations)))

    # Show enums
    if hasattr(schema, "graph_states"):
        note("GraphStates: " + ", ".join(list(schema.graph_states)))
    if hasattr(schema, "node_statuss"):
        note("NodeStatuses: " + ", ".join(list(schema.node_statuss)))

command(
    name = "test.schema",
    help = "Test schema parsing",
    flags = [],
    run = run,
)
