// SPDX-License-Identifier: MIT
// Copyright Noble Factor. All rights reserved.

//! Gitignore pattern matching extension.
//!
//! This WASM module wraps BurntSushi's `ignore` crate to provide
//! gitignore-aware path filtering for star extensions.

use ignore::gitignore::GitignoreBuilder;
use ignore::WalkBuilder;
use serde::{Deserialize, Serialize};
use std::io::{self, BufRead, Write};
use std::path::Path;

/// JSON-RPC style request from the host.
#[derive(Deserialize)]
struct Request {
    id: u64,
    method: String,
    params: serde_json::Value,
}

/// JSON-RPC style response to the host.
#[derive(Serialize)]
struct Response {
    id: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<serde_json::Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<ErrorInfo>,
}

#[derive(Serialize)]
struct ErrorInfo {
    code: i32,
    message: String,
}

/// Parameters for the `matches` method.
#[derive(Deserialize)]
struct MatchesParams {
    path: String,
    #[serde(default = "default_base")]
    base: String,
}

fn default_base() -> String {
    ".".to_string()
}

/// Parameters for the `filter` method.
#[derive(Deserialize)]
struct FilterParams {
    paths: Vec<String>,
    #[serde(default = "default_base")]
    base: String,
}

/// Result of the `matches` method.
#[derive(Serialize)]
struct MatchesResult {
    ignored: bool,
}

/// Result of the `filter` method.
#[derive(Serialize)]
struct FilterResult {
    paths: Vec<String>,
}

fn main() {
    let stdin = io::stdin();
    let mut stdout = io::stdout();

    for line in stdin.lock().lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => break,
        };

        if line.is_empty() {
            continue;
        }

        let request: Request = match serde_json::from_str(&line) {
            Ok(r) => r,
            Err(e) => {
                let resp = Response {
                    id: 0,
                    result: None,
                    error: Some(ErrorInfo {
                        code: -32700,
                        message: format!("Parse error: {}", e),
                    }),
                };
                let _ = writeln!(stdout, "{}", serde_json::to_string(&resp).unwrap());
                continue;
            }
        };

        let response = handle_request(request);
        let _ = writeln!(stdout, "{}", serde_json::to_string(&response).unwrap());
        let _ = stdout.flush();
    }
}

fn handle_request(req: Request) -> Response {
    match req.method.as_str() {
        "matches" => handle_matches(req.id, req.params),
        "filter" => handle_filter(req.id, req.params),
        _ => Response {
            id: req.id,
            result: None,
            error: Some(ErrorInfo {
                code: -32601,
                message: format!("Method not found: {}", req.method),
            }),
        },
    }
}

/// Check if a single path is ignored.
fn handle_matches(id: u64, params: serde_json::Value) -> Response {
    let params: MatchesParams = match serde_json::from_value(params) {
        Ok(p) => p,
        Err(e) => {
            return Response {
                id,
                result: None,
                error: Some(ErrorInfo {
                    code: -32602,
                    message: format!("Invalid params: {}", e),
                }),
            };
        }
    };

    let ignored = is_ignored(&params.path, &params.base);

    Response {
        id,
        result: Some(serde_json::to_value(MatchesResult { ignored }).unwrap()),
        error: None,
    }
}

/// Filter a list of paths, returning only non-ignored ones.
fn handle_filter(id: u64, params: serde_json::Value) -> Response {
    let params: FilterParams = match serde_json::from_value(params) {
        Ok(p) => p,
        Err(e) => {
            return Response {
                id,
                result: None,
                error: Some(ErrorInfo {
                    code: -32602,
                    message: format!("Invalid params: {}", e),
                }),
            };
        }
    };

    let filtered: Vec<String> = params
        .paths
        .into_iter()
        .filter(|p| !is_ignored(p, &params.base))
        .collect();

    Response {
        id,
        result: Some(serde_json::to_value(FilterResult { paths: filtered }).unwrap()),
        error: None,
    }
}

/// Check if a path is ignored by .gitignore rules.
/// Walks up the directory tree to find all .gitignore files.
fn is_ignored(path: &str, base: &str) -> bool {
    let path = Path::new(path);
    let base = Path::new(base);

    // Build gitignore matcher from base directory
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

    // Also check for .gitignore in the path's directory
    if let Some(parent) = path.parent() {
        let local_gitignore = parent.join(".gitignore");
        if local_gitignore.exists() {
            let _ = builder.add(&local_gitignore);
        }
    }

    match builder.build() {
        Ok(gitignore) => {
            let matched = gitignore.matched_path_or_any_parents(path, path.is_dir());
            matched.is_ignore()
        }
        Err(_) => false,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_matches_request() {
        let req = Request {
            id: 1,
            method: "matches".to_string(),
            params: serde_json::json!({"path": "vendor/foo.go", "base": "."}),
        };
        let resp = handle_request(req);
        assert!(resp.error.is_none());
    }

    #[test]
    fn test_filter_request() {
        let req = Request {
            id: 2,
            method: "filter".to_string(),
            params: serde_json::json!({
                "paths": ["main.go", "vendor/dep.go"],
                "base": "."
            }),
        };
        let resp = handle_request(req);
        assert!(resp.error.is_none());
    }

    #[test]
    fn test_unknown_method() {
        let req = Request {
            id: 3,
            method: "unknown".to_string(),
            params: serde_json::Value::Null,
        };
        let resp = handle_request(req);
        assert!(resp.error.is_some());
        assert_eq!(resp.error.unwrap().code, -32601);
    }
}
