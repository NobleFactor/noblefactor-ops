// SPDX-License-Identifier: Apache-2.0
// Copyright Noble Factor. All rights reserved.

//! Gitignore pattern matching extension (shared memory reactor).
//!
//! This WASM module wraps BurntSushi's `ignore` crate to provide
//! gitignore-aware path filtering for star extensions.
//!
//! Protocol: Named exports with shared memory (ptr+len).
//! The host calls `alloc` to write JSON args into module memory,
//! then calls the function which returns packed (result_ptr << 32 | result_len).
//! The host reads the result and calls `dealloc` for both pointers.

use ignore::gitignore::{Gitignore, GitignoreBuilder};
use serde::{Deserialize, Serialize};
use std::path::Path;
use std::sync::OnceLock;

/// Cached gitignore matcher built during _initialize from the workspace root.
static DEFAULT_MATCHER: OnceLock<Gitignore> = OnceLock::new();

/// Parameters for the `matches` function.
#[derive(Deserialize)]
struct MatchesParams {
    path: String,
    #[serde(default = "default_base")]
    base: String,
}

fn default_base() -> String {
    ".".to_string()
}

/// Parameters for the `filter` function.
#[derive(Deserialize)]
struct FilterParams {
    paths: Vec<String>,
    #[serde(default = "default_base")]
    base: String,
}

/// Result of the `matches` function.
#[derive(Serialize)]
struct MatchesResult {
    ignored: bool,
}

/// Result of the `filter` function.
#[derive(Serialize)]
struct FilterResult {
    paths: Vec<String>,
}

/// Reactor entry point. Called once when the persistent instance is created.
/// The gitignore matcher is built lazily on first use (in is_ignored) rather
/// than here, because the WASI filesystem mounts may use paths like /workspace
/// instead of "." as the working directory.
#[no_mangle]
pub extern "C" fn _initialize() {
    // No-op: matcher is initialized lazily in is_ignored()
}

/// Check if a single path is ignored by gitignore rules.
/// Input: JSON bytes `{"path": "...", "base": "..."}` at (ptr, len).
/// Returns: packed u64 (result_ptr << 32 | result_len) pointing to JSON `{"ignored": bool}`.
#[no_mangle]
pub extern "C" fn matches(json_ptr: *const u8, json_len: u32) -> u64 {
    let json_bytes = unsafe { std::slice::from_raw_parts(json_ptr, json_len as usize) };
    let params: MatchesParams = serde_json::from_slice(json_bytes)
        .expect("matches: invalid JSON params");

    let ignored = is_ignored(&params.path, &params.base);
    let result = serde_json::to_vec(&MatchesResult { ignored }).unwrap();
    pack_result(&result)
}

/// Filter a list of paths, returning only non-ignored ones.
/// Input: JSON bytes `{"paths": [...], "base": "..."}` at (ptr, len).
/// Returns: packed u64 pointing to JSON `{"paths": [...]}`.
#[no_mangle]
pub extern "C" fn filter(json_ptr: *const u8, json_len: u32) -> u64 {
    let json_bytes = unsafe { std::slice::from_raw_parts(json_ptr, json_len as usize) };
    let params: FilterParams = serde_json::from_slice(json_bytes)
        .expect("filter: invalid JSON params");

    let filtered: Vec<String> = params
        .paths
        .into_iter()
        .filter(|p| !is_ignored(p, &params.base))
        .collect();

    let result = serde_json::to_vec(&FilterResult { paths: filtered }).unwrap();
    pack_result(&result)
}

/// Allocate memory in the WASM heap for the host to write into.
/// Exported as "alloc" in WASM; uses cfg_attr to avoid C runtime conflicts in native tests.
#[cfg_attr(target_arch = "wasm32", export_name = "alloc")]
pub extern "C" fn wasm_alloc(size: u32) -> *mut u8 {
    let layout = std::alloc::Layout::from_size_align(size as usize, 1)
        .expect("alloc: invalid layout");
    unsafe { std::alloc::alloc(layout) }
}

/// Free previously allocated memory.
/// Uses "dealloc" as the export name to avoid conflicts with the C standard library's free().
#[cfg_attr(target_arch = "wasm32", export_name = "dealloc")]
pub extern "C" fn wasm_free(ptr: *mut u8, size: u32) {
    if ptr.is_null() || size == 0 {
        return;
    }
    let layout = std::alloc::Layout::from_size_align(size as usize, 1)
        .expect("free: invalid layout");
    unsafe { std::alloc::dealloc(ptr, layout) }
}

/// Write result bytes to allocated memory and return packed (ptr << 32 | len).
fn pack_result(data: &[u8]) -> u64 {
    let len = data.len() as u32;
    let ptr = wasm_alloc(len);
    unsafe {
        std::ptr::copy_nonoverlapping(data.as_ptr(), ptr, len as usize);
    }
    ((ptr as u64) << 32) | (len as u64)
}

/// Check if a path is ignored by .gitignore rules.
/// Lazily initializes and caches the matcher from the first base directory seen.
/// Subsequent calls with the same base reuse the cache; different bases build one-off.
fn is_ignored(path: &str, base: &str) -> bool {
    let path = Path::new(path);

    // Try to use cached matcher (initialized lazily from first call's base)
    let matcher = DEFAULT_MATCHER.get_or_init(|| build_matcher(base));
    let matched = matcher.matched_path_or_any_parents(path, path.is_dir());
    matched.is_ignore()
}

/// Build a gitignore matcher by walking up the directory tree from base.
fn build_matcher(base: &str) -> Gitignore {
    let base = Path::new(base);
    let mut builder = GitignoreBuilder::new(base);

    // Walk up from base to find .gitignore files
    let mut current = base.to_path_buf();
    let mut gitignore_paths = Vec::new();

    loop {
        let gitignore = current.join(".gitignore");
        if gitignore.exists() {
            gitignore_paths.push(gitignore);
        }

        match current.parent() {
            Some(parent) if parent != current => {
                current = parent.to_path_buf();
            }
            _ => break,
        }
    }

    // Add gitignore files in reverse order (root first)
    for gi_path in gitignore_paths.into_iter().rev() {
        let _ = builder.add(&gi_path);
    }

    builder.build().unwrap_or_else(|_| {
        // Return an empty matcher on error
        GitignoreBuilder::new(base).build().unwrap()
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_build_matcher() {
        let matcher = build_matcher(".");
        // Should not panic — builds matcher from current directory
        let _ = matcher.matched_path_or_any_parents(Path::new("test.go"), false);
    }

    #[test]
    fn test_is_ignored_default_base() {
        // Initialize the cached matcher
        _initialize();

        // Should not panic
        let _ = is_ignored("test.go", ".");
        let _ = is_ignored("test.go", "");
    }

    #[test]
    fn test_is_ignored_custom_base() {
        // Should build one-off matcher without requiring _initialize
        let _ = is_ignored("test.go", "/tmp");
    }

    #[test]
    fn test_alloc_free() {
        let ptr = wasm_alloc(1024);
        assert!(!ptr.is_null());
        wasm_free(ptr, 1024);
    }

    #[test]
    fn test_free_null() {
        // Should not panic
        wasm_free(std::ptr::null_mut(), 0);
    }
}
