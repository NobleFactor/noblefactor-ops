;; Simple echo module for testing
;; Reads from stdin, writes to stdout
(module
  ;; Import WASI functions
  (import "wasi_snapshot_preview1" "fd_read"
    (func $fd_read (param i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "fd_write"
    (func $fd_write (param i32 i32 i32 i32) (result i32)))

  ;; Memory export (required by WASI)
  (memory (export "memory") 1)

  ;; IO vector for fd_read/fd_write: ptr=0, len=4096
  (data (i32.const 0) "\08\00\00\00")  ;; iov_base = 8
  (data (i32.const 4) "\00\10\00\00")  ;; iov_len = 4096

  ;; _initialize - reactor initialization (does nothing)
  (func (export "_initialize"))

  ;; echo - read stdin and write to stdout
  (func (export "echo") (result i32)
    (local $nread i32)

    ;; Read from stdin (fd=0) into buffer at offset 8
    ;; fd_read(fd, iovs, iovs_len, nread_ptr)
    (call $fd_read
      (i32.const 0)    ;; fd = stdin
      (i32.const 0)    ;; iovs pointer
      (i32.const 1)    ;; iovs count
      (i32.const 4100) ;; where to store bytes read
    )
    drop

    ;; Get bytes read
    (local.set $nread (i32.load (i32.const 4100)))

    ;; Update iov_len to actual bytes read
    (i32.store (i32.const 4) (local.get $nread))

    ;; Write to stdout (fd=1)
    (call $fd_write
      (i32.const 1)    ;; fd = stdout
      (i32.const 0)    ;; iovs pointer
      (i32.const 1)    ;; iovs count
      (i32.const 4100) ;; where to store bytes written
    )
    drop

    ;; Return bytes written
    (i32.load (i32.const 4100))
  )
)
