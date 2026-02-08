---
title: "Devlore Extension Model"
description: "Architecture for extending devlore CLIs with new capabilities via YAML specs, Starlark commands, and Go bindings"
status: draft
created: 2025-02-07
updated: 2025-02-07
---

# Devlore Extension Model

This document defines how to extend devlore CLIs with new capabilities.

## Extension Specification

An extension is described by a YAML specification. The `lint.copyright` extension serves as the canonical example throughout this document.

```yaml
extension: lint.copyright

command:
  help: |
    Check or fix copyright headers in source files.

    Ensures all source files have correct SPDX license headers.
    Supports Go, Starlark, and shell files.

    Examples:
      star lint copyright              # Check headers
      star lint copyright --fix        # Fix headers
      star lint copyright --include="src/**/*.go"

flags:
  - name: fix
    type: bool
    default: false
    help: Add missing headers, update incorrect ones
  - name: include
    type: glob
    default: "**/*.{go,star,sh}"
    help: Files to check
  - name: exclude
    type: glob
    default: "**/vendor/**"
    help: Files to skip

config:
  type: CopyrightConfig
  fields:
    enabled: bool
    license: string
    holder: string
    patterns: map[string]Pattern
    exclude: []string

  Pattern:
    match: string
    replace: string

  defaults:
    enabled: false
    license: "auto"
    holder: ""
    patterns:
      "*.go":
        match: '// SPDX-License-Identifier: \S+\s*\n// Copyright.*'
        replace: "// SPDX-License-Identifier: {license}\n// Copyright {holder}. All rights reserved."
      "*.star":
        match: '# SPDX-License-Identifier: \S+\s*\n# Copyright.*'
        replace: "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved."
      "*.sh;*.bash;*.fish;*.zsh":
        match: '# SPDX-License-Identifier: \S+\s*\n# Copyright.*'
        replace: "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved."
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
```

### Naming Convention

The extension name determines command and config paths:

| Element | Value | Derivation |
|---------|-------|------------|
| Extension | `lint.copyright` | Declared name |
| Command | `star lint copyright` | Extension name with dots as subcommands |
| Config path | `lint.copyright` | Same as extension name |
| Env var prefix | `STAR_LINT_COPYRIGHT_` | `STAR_` + extension name (dots→underscores, uppercase) |

### Flag Resolution

Each flag resolves in priority order:

1. CLI argument: `--fix`
2. Environment variable: `STAR_LINT_COPYRIGHT_FIX`
3. Config file: `lint.copyright.fix`
4. Default: `false`

## Extension Components

An extension provides three components:

| Component | Language | Purpose |
|-----------|----------|---------|
| **Binding Functions** | Go | Low-level primitives exposed to Starlark |
| **Config Schema** | Starlark | Typed configuration with defaults |
| **Command** | Starlark | CLI subcommand implementation |

## Binding Functions (Go)

Binding functions provide low-level primitives that Starlark scripts call. They are implemented as methods on a receiver type.

### Receiver Pattern

A receiver is a Go struct whose exported methods become Starlark module functions:

```go
// internal/starlark/builtin_copyright.go

// CopyrightChecker provides copyright header checking and fixing.
type CopyrightChecker struct{}

// Check verifies files have correct copyright headers.
// Exposed as copyright.check() in Starlark.
func (c *CopyrightChecker) Check(
    _ *starlark.Thread,
    _ *starlark.Builtin,
    args starlark.Tuple,
    kwargs []starlark.Tuple,
) (starlark.Value, error) {
    var paths *starlark.List
    var license, holder string
    var patterns *starlark.Dict

    if err := starlark.UnpackArgs("copyright.check", args, kwargs,
        "paths", &paths,
        "license", &license,
        "holder", &holder,
        "patterns", &patterns,
    ); err != nil {
        return nil, err
    }

    // Implementation...

    return starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
        "issues": starlark.NewList(issues),
        "passed": starlark.Bool(len(issues) == 0),
        "count":  starlark.MakeInt(len(issues)),
    }), nil
}

// Fix adds or updates copyright headers.
// Exposed as copyright.fix() in Starlark.
func (c *CopyrightChecker) Fix(...) (starlark.Value, error) { ... }

// DetectLicense detects SPDX identifier from LICENSE file.
// Exposed as copyright.detect_license() in Starlark.
func (c *CopyrightChecker) DetectLicense(...) (starlark.Value, error) { ... }

func init() {
    RegisterReceiver("copyright", &CopyrightChecker{})
}
```

### Method Signature

Methods must match this exact signature to be exposed:

```go
func (r *Receiver) MethodName(
    thread *starlark.Thread,
    b *starlark.Builtin,
    args starlark.Tuple,
    kwargs []starlark.Tuple,
) (starlark.Value, error)
```

### Name Conversion

Method names are converted from CamelCase to snake_case:

| Go Method | Starlark Function |
|-----------|-------------------|
| `Check` | `copyright.check` |
| `Fix` | `copyright.fix` |
| `DetectLicense` | `copyright.detect_license` |

## Config Schema (Starlark)

Extensions declare their configuration using `config.define()`. The config path matches the extension name.

```python
# ops/lint-copyright.star

config.define("lint.copyright", struct(
    enabled = False,
    license = "auto",
    holder = "",
    patterns = {
        "*.go": struct(
            match = r'// SPDX-License-Identifier: \S+\s*\n// Copyright.*',
            replace = "// SPDX-License-Identifier: {license}\n// Copyright {holder}. All rights reserved.",
        ),
        "*.star": struct(
            match = r'# SPDX-License-Identifier: \S+\s*\n# Copyright.*',
            replace = "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
        ),
        "*.sh;*.bash;*.fish;*.zsh": struct(
            match = r'# SPDX-License-Identifier: \S+\s*\n# Copyright.*',
            replace = "# SPDX-License-Identifier: {license}\n# Copyright {holder}. All rights reserved.",
        ),
    },
    exclude = ["**/vendor/**", "**/testdata/**"],
))
```

### User Configuration

Users override defaults in `star.yaml`:

```yaml
lint:
  copyright:
    enabled: true
    license: MIT
    holder: Noble Factor
    exclude:
      - "**/vendor/**"
      - "**/testdata/**"
      - "**/*_generated.go"
```

### Accessing Config

```python
cfg = config.get().lint.copyright

if cfg.enabled:
    license = cfg.license
    holder = cfg.holder
    for pattern, rule in cfg.patterns.items():
        # pattern: "*.go"
        # rule.match: regex
        # rule.replace: template
```

## Command (Starlark)

Commands are registered using the `command()` builtin. The command name matches the extension name.

```python
# ops/lint-copyright.star

def run_copyright(ctx):
    cfg = config.get().lint.copyright

    if not cfg.enabled:
        warn("Copyright checking disabled")
        warn("Set lint.copyright.enabled: true in star.yaml")
        return

    fix = ctx.args.get("fix") == "true"
    include = ctx.args.get("include")
    exclude = ctx.args.get("exclude")

    # Detect license if auto
    license = cfg.license
    if license == "auto":
        result = copyright.detect_license("LICENSE")
        if result.detected:
            license = result.license
        else:
            fail("Cannot detect license. Set lint.copyright.license in star.yaml")

    holder = cfg.holder
    if not holder:
        fail("Set lint.copyright.holder in star.yaml")

    # Collect files
    files = fs.glob(include)
    files = [f for f in files if not matches_glob(f, exclude)]
    files = [f for f in files if not matches_any(f, cfg.exclude)]

    if fix:
        result = copyright.fix(
            paths = files,
            license = license,
            holder = holder,
            patterns = cfg.patterns,
        )
        if result.count > 0:
            success("Fixed " + str(result.count) + " files")
        else:
            success("All files have correct headers")
    else:
        result = copyright.check(
            paths = files,
            license = license,
            holder = holder,
            patterns = cfg.patterns,
        )
        if result.passed:
            success("All " + str(len(files)) + " files have correct headers")
        else:
            for issue in result.issues:
                error(issue.file + ": " + issue.message)
            fail("Found " + str(result.count) + " files with issues")

command(
    name = "lint.copyright",
    help = "Check or fix copyright headers in source files",
    flags = [
        {"name": "fix", "type": "bool", "default": "false",
         "help": "Add missing headers, update incorrect ones"},
        {"name": "include", "type": "glob", "default": "**/*.{go,star,sh}",
         "help": "Files to check"},
        {"name": "exclude", "type": "glob", "default": "**/vendor/**",
         "help": "Files to skip"},
    ],
    run = run_copyright,
)
```

## Runtime Context

When an extension command executes, the runtime provides a context object with access to resolved flags, configuration, and metadata.

### Context Structure

```python
def run_copyright(ctx):
    # Flags - resolved via CLI → ENV → Config → Default
    fix = ctx.args.get("fix") == "true"
    include = ctx.args.get("include")

    # Configuration - merged defaults + star.yaml
    cfg = config.get().lint.copyright
    license = cfg.license
    holder = cfg.holder

    # Metadata - global runtime state
    if ctx.dry_run:
        note("Would fix files...")
```

### What the Runtime Provides

| Component | Source | Access |
|-----------|--------|--------|
| **Flags** | Extension spec `flags:` section | `ctx.args.get("flag_name")` |
| **Config** | Extension spec `config:` + `star.yaml` | `config.get().lint.copyright` |
| **Metadata** | Global flags (`--dry-run`) | `ctx.dry_run` |

### Flag Resolution

The runtime resolves each flag through the resolution chain before the command runs:

1. **CLI argument**: `--fix` → highest priority
2. **Environment variable**: `STAR_LINT_COPYRIGHT_FIX`
3. **Config file**: `lint.copyright.fix` in `star.yaml`
4. **Default**: From extension spec `flags[].default`

All resolved values are strings in `ctx.args`. Boolean flags use `"true"` / `"false"`.

### Config Access

Configuration is accessed via `config.get()`, which returns a Starlark struct:

```python
cfg = config.get().lint.copyright

# Scalar fields - attribute access
enabled = cfg.enabled      # bool
license = cfg.license      # string
holder = cfg.holder        # string

# List fields - iteration
for pattern in cfg.exclude:
    print(pattern)

# Map fields - .items() or .get()
for glob, rule in cfg.patterns.items():
    print(glob, rule.match, rule.replace)
```

The config merges:
1. Extension defaults (from `config.define()`)
2. User overrides (from `star.yaml`)

## Configuration Architecture

Configuration is strongly typed in Go but accessible from Starlark via an adapter interface. The architecture separates concerns: Go types remain pure Go, and a boundary adapter handles Starlark interop.

### ConfigElement

The base type embedded by all config sections. Supports hierarchical composition via `Register`.

```go
// ConfigElement is the base type embedded by all config sections.
type ConfigElement struct {
    path     string
    children map[string]interface{}  // child ConfigElements
}

func (e *ConfigElement) Path() string { return e.path }

// Register adds a child ConfigElement to this element.
// The child's path is computed relative to this element.
func (e *ConfigElement) Register(name string, child interface{}) {
    if e.children == nil {
        e.children = make(map[string]interface{})
    }

    // Set child's path
    childPath := name
    if e.path != "" {
        childPath = e.path + "." + name
    }
    setPath(child, childPath)

    e.children[name] = child
}

// Get retrieves a child by name.
func (e *ConfigElement) Get(name string) interface{} {
    return e.children[name]
}

// Navigate traverses the config hierarchy by dotted path.
func (e *ConfigElement) Navigate(path string) interface{} {
    if path == "" {
        return e
    }

    parts := strings.Split(path, ".")
    current := e

    for i, part := range parts {
        child := current.Get(part)
        if child == nil {
            return nil
        }

        // If this is the last part, return the child
        if i == len(parts)-1 {
            return child
        }

        // If child is a ConfigElement, continue traversing
        if elem, ok := child.(*ConfigElement); ok {
            current = elem
            continue
        }

        // If child embeds ConfigElement, extract it
        rv := reflect.ValueOf(child).Elem()
        if field := rv.FieldByName("ConfigElement"); field.IsValid() {
            current = field.Addr().Interface().(*ConfigElement)
            continue
        }

        // Child is a leaf struct - traverse its fields
        for _, remaining := range parts[i+1:] {
            field := rv.FieldByName(toPascalCase(remaining))
            if !field.IsValid() {
                return nil
            }
            rv = field
        }
        return rv.Interface()
    }

    return current
}
```

### Extension Config Types

Extension configs are pure Go structs that embed ConfigElement:

```go
type CopyrightConfig struct {
    ConfigElement
    Enabled  bool
    License  string
    Holder   string
    Patterns map[string]Pattern
    Exclude  []string
}

type Pattern struct {
    Match   string
    Replace string
}
```

### Config (Root)

The root of the configuration hierarchy. Embeds ConfigElement with `path = ""`, names the source, and implements core operations.

```go
// Config is the root of the configuration hierarchy.
type Config struct {
    ConfigElement                      // path = "", children = top-level sections
    source   string                    // filename, resource URL, etc.
    dirty    bool                      // modified since load
}

// Source returns the configuration source path.
func (c *Config) Source() string {
    return c.source
}

// IsDirty returns true if config has been modified since load.
func (c *Config) IsDirty() bool {
    return c.dirty
}

// Accessor returns a typed accessor for a section.
func (c *Config) Accessor(path string) *ConfigAccessor {
    elem := c.Navigate(path)
    return &ConfigAccessor{v: reflect.ValueOf(elem).Elem()}
}

// ToStarlark wraps the config for Starlark access.
func (c *Config) ToStarlark() starlark.Value {
    return &ConfigValue{elem: c}
}
```

### Hierarchy Construction

The hierarchy is built through `Register`, starting at the root:

```go
// Root config (path = "")
cfg := &Config{
    ConfigElement: ConfigElement{path: ""},
    source:        "star.yaml",
}

// Register "lint" section
lint := &LintConfig{}
cfg.Register("lint", lint)  // lint.path = "lint"

// Register "lint.copyright" section
copyright := &CopyrightConfig{Enabled: false, License: "auto"}
lint.Register("copyright", copyright)  // copyright.path = "lint.copyright"
```

### Navigation

Dot notation navigation through the hierarchy:

```go
cfg, _ := config.Load("star.yaml")

// Navigate to section
copyright := cfg.Navigate("lint.copyright")

// Navigate to field
enabled := cfg.Navigate("lint.copyright.enabled").(bool)

// Typed accessor
acc := cfg.Accessor("lint.copyright")
if acc.Bool("enabled") {
    license := acc.String("license")
}
```

### ConfigValue (Starlark Adapter)

Wraps any ConfigElement to implement `starlark.HasAttrs`. The boundary crossing happens here.

```go
// ConfigValue adapts Go config for Starlark access
type ConfigValue struct {
    elem interface{}  // any ConfigElement
}

func (v *ConfigValue) String() string        { return fmt.Sprintf("config(%T)", v.elem) }
func (v *ConfigValue) Type() string          { return "config" }
func (v *ConfigValue) Freeze()               {}
func (v *ConfigValue) Truth() starlark.Bool  { return true }
func (v *ConfigValue) Hash() (uint32, error) { return 0, errors.New("unhashable") }

// Attr provides attribute access for Starlark (cfg.enabled, cfg.license)
func (v *ConfigValue) Attr(name string) (starlark.Value, error) {
    rv := reflect.ValueOf(v.elem).Elem()
    field := rv.FieldByName(toPascalCase(name))
    if !field.IsValid() {
        return nil, starlark.NoSuchAttrError(name)
    }
    return goToStarlark(field.Interface())
}

func (v *ConfigValue) AttrNames() []string {
    // Reflect over exported fields, convert to snake_case
}
```

### Boundary Crossing

Go code uses native types. Starlark receives wrapped values. The dance happens at the boundary, not throughout the codebase.

```go
// Wrap at the Starlark boundary
func ToStarlark(cfg interface{}) starlark.Value {
    return &ConfigValue{elem: cfg}
}

// Go code - native access
copyright := config.Get[*CopyrightConfig]("lint.copyright")
if copyright.Enabled {
    license := copyright.License
}

// Starlark boundary - wrap once
predeclared["config"] = ToStarlark(root)
```

```python
# Starlark - Attr() called under the hood
cfg = config.lint.copyright
if cfg.enabled:
    license = cfg.license
```

### Runtime Type Generation

Go struct types are generated at runtime from YAML extension specs using `reflect.StructOf`. No code generation step required.

```go
import "reflect"

// generateConfigType creates a Go struct type from YAML schema
func generateConfigType(spec ConfigSpec) reflect.Type {
    var fields []reflect.StructField

    for _, f := range spec.Fields {
        fields = append(fields, reflect.StructField{
            Name: toPascalCase(f.Name),
            Type: goTypeFor(f.Type),
            Tag:  reflect.StructTag(fmt.Sprintf(`yaml:"%s"`, f.Name)),
        })
    }

    return reflect.StructOf(fields)
}

func goTypeFor(typeName string) reflect.Type {
    switch typeName {
    case "bool":
        return reflect.TypeOf(true)
    case "string":
        return reflect.TypeOf("")
    case "int":
        return reflect.TypeOf(0)
    case "[]string":
        return reflect.TypeOf([]string{})
    default:
        // Nested struct - recurse
        return generateConfigType(nestedSpec)
    }
}
```

### Type Cache

Generated types are cached by extension path:

```go
var typeCache = struct {
    sync.RWMutex
    types map[string]reflect.Type
}{types: make(map[string]reflect.Type)}

func getOrCreateType(path string, spec ConfigSpec) reflect.Type {
    typeCache.RLock()
    if typ, ok := typeCache.types[path]; ok {
        typeCache.RUnlock()
        return typ
    }
    typeCache.RUnlock()

    typeCache.Lock()
    defer typeCache.Unlock()

    // Double-check after acquiring write lock
    if typ, ok := typeCache.types[path]; ok {
        return typ
    }

    typ := generateConfigType(spec)
    typeCache.types[path] = typ
    return typ
}
```

### Instance Creation

```go
func newConfigInstance(typ reflect.Type, defaults map[string]any) interface{} {
    ptr := reflect.New(typ)
    instance := ptr.Elem()

    for name, val := range defaults {
        field := instance.FieldByName(toPascalCase(name))
        field.Set(reflect.ValueOf(val))
    }

    return ptr.Interface()
}
```

### ConfigAccessor

Wraps reflection in typed accessors for Go code:

```go
type ConfigAccessor struct {
    v reflect.Value
}

func (a *ConfigAccessor) Bool(name string) bool {
    return a.v.FieldByName(toPascalCase(name)).Bool()
}

func (a *ConfigAccessor) String(name string) string {
    return a.v.FieldByName(toPascalCase(name)).String()
}

func (a *ConfigAccessor) Int(name string) int {
    return int(a.v.FieldByName(toPascalCase(name)).Int())
}

func (a *ConfigAccessor) Struct(name string) *ConfigAccessor {
    return &ConfigAccessor{v: a.v.FieldByName(toPascalCase(name))}
}
```

Usage:

```go
cfg := config.Get("lint.copyright")
accessor := &ConfigAccessor{v: reflect.ValueOf(cfg).Elem()}

if accessor.Bool("enabled") {
    license := accessor.String("license")
    holder := accessor.String("holder")
}
```

### Loading

```go
// Load reads configuration from the source.
func Load(source string) (*Config, error) {
    cfg := &Config{
        ConfigElement: ConfigElement{path: ""},
        source:        source,
    }

    // 1. Register extension defaults (builds hierarchy)
    for path, spec := range extensionRegistry {
        cfg.registerExtension(path, spec)
    }

    // 2. Load source file (e.g., star.yaml)
    data, err := os.ReadFile(source)
    if err != nil && !os.IsNotExist(err) {
        return nil, err
    }

    // 3. Parse and merge into hierarchy
    if data != nil {
        var raw map[string]interface{}
        yaml.Unmarshal(data, &raw)
        cfg.mergeRaw(raw)
    }

    return cfg, nil
}

// registerExtension registers an extension at a dotted path.
// Creates intermediate elements as needed.
func (c *Config) registerExtension(path string, spec ConfigSpec) {
    parts := strings.Split(path, ".")
    current := &c.ConfigElement

    // Navigate/create intermediate elements
    for _, part := range parts[:len(parts)-1] {
        child := current.Get(part)
        if child == nil {
            child = &ConfigElement{}
            current.Register(part, child)
        }
        current = child.(*ConfigElement)
    }

    // Create and register the leaf element
    typ := getOrCreateType(path, spec)
    instance := newConfigInstance(typ, spec.Defaults)
    current.Register(parts[len(parts)-1], instance)
}
```

### Saving

```go
// Save writes configuration back to the source.
func (c *Config) Save() error {
    data, err := yaml.Marshal(c.toMap())
    if err != nil {
        return err
    }
    c.dirty = false
    return os.WriteFile(c.source, data, 0644)
}
```

### Registration from YAML

Extensions can be registered directly from their YAML specification. This function shows the complete transformation from YAML config description to ConfigElement hierarchy.

```go
// ExtensionSpec represents a parsed extension YAML file.
type ExtensionSpec struct {
    Extension string      `yaml:"extension"`
    Command   CommandSpec `yaml:"command"`
    Flags     []FlagSpec  `yaml:"flags"`
    Config    ConfigSpec  `yaml:"config"`
}

// ConfigSpec represents the config section of an extension YAML.
type ConfigSpec struct {
    Type     string                 `yaml:"type"`
    Fields   map[string]string      `yaml:"fields"`    // field name → type
    Nested   map[string]ConfigSpec  `yaml:"-"`         // nested type definitions
    Defaults map[string]any         `yaml:"defaults"`
}

// RegisterFromYAML parses an extension YAML and registers its config
// in the ConfigElement hierarchy.
func RegisterFromYAML(root *Config, yamlPath string) error {
    // 1. Parse extension YAML
    data, err := os.ReadFile(yamlPath)
    if err != nil {
        return fmt.Errorf("read extension spec: %w", err)
    }

    var spec ExtensionSpec
    if err := yaml.Unmarshal(data, &spec); err != nil {
        return fmt.Errorf("parse extension spec: %w", err)
    }

    // 2. Parse nested type definitions from YAML
    //    (Pattern, Rule, etc. defined at same level as fields)
    spec.Config.Nested = parseNestedTypes(data)

    // 3. Generate Go type from config schema
    typ := generateConfigTypeFromSpec(spec.Config)

    // 4. Create instance with defaults
    instance := newConfigInstanceFromSpec(typ, spec.Config.Defaults)

    // 5. Register in hierarchy at the extension path
    registerAtPath(root, spec.Extension, instance)

    return nil
}

// generateConfigTypeFromSpec creates a reflect.Type from ConfigSpec.
// Handles nested types (Pattern, Rule, etc.) recursively.
func generateConfigTypeFromSpec(spec ConfigSpec) reflect.Type {
    var fields []reflect.StructField

    // Add embedded ConfigElement as first field
    fields = append(fields, reflect.StructField{
        Name:      "ConfigElement",
        Type:      reflect.TypeOf(ConfigElement{}),
        Anonymous: true,
    })

    // Add fields from spec
    for name, typeName := range spec.Fields {
        fields = append(fields, reflect.StructField{
            Name: toPascalCase(name),
            Type: resolveType(typeName, spec.Nested),
            Tag:  reflect.StructTag(fmt.Sprintf(`yaml:"%s"`, name)),
        })
    }

    return reflect.StructOf(fields)
}

// resolveType converts a type name string to reflect.Type.
// Handles primitives, slices, maps, and nested struct types.
func resolveType(typeName string, nested map[string]ConfigSpec) reflect.Type {
    switch {
    case typeName == "bool":
        return reflect.TypeOf(true)
    case typeName == "string":
        return reflect.TypeOf("")
    case typeName == "int":
        return reflect.TypeOf(0)
    case strings.HasPrefix(typeName, "[]"):
        elemType := resolveType(typeName[2:], nested)
        return reflect.SliceOf(elemType)
    case strings.HasPrefix(typeName, "map["):
        // Parse map[KeyType]ValueType
        keyEnd := strings.Index(typeName, "]")
        keyType := resolveType(typeName[4:keyEnd], nested)
        valType := resolveType(typeName[keyEnd+1:], nested)
        return reflect.MapOf(keyType, valType)
    default:
        // Check if it's a nested type definition
        if nestedSpec, ok := nested[typeName]; ok {
            return generateConfigTypeFromSpec(nestedSpec)
        }
        // Unknown type - fall back to interface{}
        return reflect.TypeOf((*any)(nil)).Elem()
    }
}

// newConfigInstanceFromSpec creates an instance and populates defaults.
// Handles nested structs and maps recursively.
func newConfigInstanceFromSpec(typ reflect.Type, defaults map[string]any) interface{} {
    ptr := reflect.New(typ)
    instance := ptr.Elem()

    for name, val := range defaults {
        field := instance.FieldByName(toPascalCase(name))
        if !field.IsValid() || !field.CanSet() {
            continue
        }
        setFieldValue(field, val)
    }

    return ptr.Interface()
}

// setFieldValue sets a reflect.Value from an any value.
// Recursively handles maps and nested structs.
func setFieldValue(field reflect.Value, val any) {
    switch v := val.(type) {
    case map[string]any:
        if field.Kind() == reflect.Map {
            // Map field - create map and populate
            mapType := field.Type()
            newMap := reflect.MakeMap(mapType)
            for k, mv := range v {
                keyVal := reflect.ValueOf(k)
                elemVal := reflect.New(mapType.Elem()).Elem()
                if mvMap, ok := mv.(map[string]any); ok {
                    setStructFields(elemVal, mvMap)
                } else {
                    elemVal.Set(reflect.ValueOf(mv))
                }
                newMap.SetMapIndex(keyVal, elemVal)
            }
            field.Set(newMap)
        } else if field.Kind() == reflect.Struct {
            // Nested struct
            setStructFields(field, v)
        }
    case []any:
        // Slice field
        sliceType := field.Type()
        newSlice := reflect.MakeSlice(sliceType, len(v), len(v))
        for i, elem := range v {
            elemVal := newSlice.Index(i)
            setFieldValue(elemVal, elem)
        }
        field.Set(newSlice)
    default:
        field.Set(reflect.ValueOf(val).Convert(field.Type()))
    }
}

// setStructFields populates struct fields from a map.
func setStructFields(structVal reflect.Value, values map[string]any) {
    for name, val := range values {
        field := structVal.FieldByName(toPascalCase(name))
        if field.IsValid() && field.CanSet() {
            setFieldValue(field, val)
        }
    }
}

// registerAtPath registers a config instance at a dotted path.
// Creates intermediate ConfigElement nodes as needed.
func registerAtPath(root *Config, path string, instance interface{}) {
    parts := strings.Split(path, ".")
    current := &root.ConfigElement

    // Navigate/create intermediate elements
    for _, part := range parts[:len(parts)-1] {
        child := current.Get(part)
        if child == nil {
            intermediate := &ConfigElement{}
            current.Register(part, intermediate)
            child = intermediate
        }
        if elem, ok := child.(*ConfigElement); ok {
            current = elem
        } else {
            // Child embeds ConfigElement - extract it
            rv := reflect.ValueOf(child).Elem()
            current = rv.FieldByName("ConfigElement").Addr().Interface().(*ConfigElement)
        }
    }

    // Register the leaf instance
    current.Register(parts[len(parts)-1], instance)
}

// parseNestedTypes extracts nested type definitions from extension YAML.
// Types like Pattern, Rule are defined alongside fields in the config section.
func parseNestedTypes(yamlData []byte) map[string]ConfigSpec {
    nested := make(map[string]ConfigSpec)

    // Parse raw YAML to find type definitions
    var raw map[string]any
    yaml.Unmarshal(yamlData, &raw)

    if config, ok := raw["config"].(map[string]any); ok {
        for key, val := range config {
            // Skip known keys
            if key == "type" || key == "fields" || key == "defaults" {
                continue
            }
            // This is a nested type definition
            if typeDef, ok := val.(map[string]any); ok {
                fields := make(map[string]string)
                for fieldName, fieldType := range typeDef {
                    if ft, ok := fieldType.(string); ok {
                        fields[fieldName] = ft
                    }
                }
                nested[key] = ConfigSpec{
                    Type:   key,
                    Fields: fields,
                }
            }
        }
    }

    return nested
}

// toPascalCase converts snake_case to PascalCase.
func toPascalCase(s string) string {
    parts := strings.Split(s, "_")
    for i, p := range parts {
        if len(p) > 0 {
            parts[i] = strings.ToUpper(p[:1]) + p[1:]
        }
    }
    return strings.Join(parts, "")
}
```

Example usage:

```go
// At startup, register all extensions from ops/*.yaml
root := &Config{ConfigElement: ConfigElement{path: ""}, source: "star.yaml"}

files, _ := filepath.Glob("ops/*.yaml")
for _, f := range files {
    if err := RegisterFromYAML(root, f); err != nil {
        log.Printf("warning: %s: %v", f, err)
    }
}

// Now the hierarchy is populated:
// root
//   └── lint (ConfigElement)
//         └── copyright (CopyrightConfig - generated type)
//               ├── Enabled: false
//               ├── License: "auto"
//               ├── Holder: ""
//               ├── Patterns: map[string]Pattern{...}
//               └── Exclude: []string{...}
```

## Implementation Details

### Receiver Registration

```go
// internal/starlark/receiver.go

var receiverRegistry = make(map[string]interface{})

// RegisterReceiver registers a Go type as a Starlark module.
func RegisterReceiver(name string, receiver interface{})

// GetReceiverModule returns a Starlark module for a registered receiver.
func GetReceiverModule(name string) *starlarkstruct.Module
```

### Reflection-Based Wrapping

The receiver system uses reflection to:

1. Enumerate exported methods on the receiver type
2. Filter methods matching the builtin signature
3. Wrap each method as a Starlark builtin
4. Build a module with snake_case function names

```go
func buildReceiverModule(name string, receiver interface{}) *starlarkstruct.Module {
    members := starlark.StringDict{}
    rv := reflect.ValueOf(receiver)
    rt := rv.Type()

    for i := 0; i < rt.NumMethod(); i++ {
        method := rt.Method(i)
        if !method.IsExported() || !isBuiltinCompatible(method.Type) {
            continue
        }
        starName := toSnakeCase(method.Name)
        members[starName] = wrapMethod(name+"."+starName, rv.Method(i))
    }

    return &starlarkstruct.Module{Name: name, Members: members}
}
```

## Files

| File | Purpose |
|------|---------|
| `internal/config/config.go` | Config, ConfigElement, loading/saving |
| `internal/config/accessor.go` | ConfigAccessor typed access |
| `internal/config/starlark.go` | ConfigValue Starlark adapter |
| `internal/config/types.go` | Runtime type generation, cache |
| `internal/starlark/receiver.go` | Receiver registration and reflection |
| `internal/starlark/builtin_copyright.go` | CopyrightChecker receiver |
| `internal/starlark/builtin_config.go` | Config module with `define()` |
| `ops/lint-copyright.star` | Copyright extension implementation |

## Design Principles

1. **Starlark-first** — Config and commands are defined in Starlark
2. **Go escape hatch** — Binding functions for performance/system access
3. **Convention over configuration** — Extension name determines paths
4. **One receiver = one namespace** — `CopyrightChecker` → `copyright.*`
5. **Boundary crossing** — Go stays Go; adapter handles Starlark interop
6. **Runtime types** — YAML specs generate Go types via reflection
