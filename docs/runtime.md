# Runtime helpers

Two small packages every cocoonstack binary wires into `main`.

## httpx

`httpx.Run(ctx, shutdownTimeout, specs...)` starts one or more servers, then
shuts them all down together when the context is canceled or any one of them
fails to serve. It returns the joined serve and shutdown errors.

```go
srv := httpx.NewServer(":8443", mux)
srv.TLSConfig = tlsCfg

err := httpx.Run(ctx, 10*time.Second,
    httpx.HTTPServerSpec(httpx.NewServer(":9090", metricsMux)),
    httpx.HTTPSServerSpec(srv),
)
```

`httpx.NewServer` applies `DefaultReadHeaderTimeout` (10s), which caps client
header send time to mitigate Slowloris. `HTTPSServerSpec` calls
`ListenAndServeTLS("", "")`, so the certificate comes from `srv.TLSConfig` —
pair it with [`k8s.LoadOrGenerateCert`](kubernetes.md#tls-bring-up).

Two ordering details matter. Shutdown runs on a context derived with
`context.WithoutCancel`, so it still has its full timeout after the parent
context fires. And a `Start` failure cancels the run context itself, so a bind
or TLS error at startup surfaces immediately instead of hanging until SIGTERM.

## log

```go
if err := log.Setup(ctx, "COCOON_OPERATOR_LOG_LEVEL"); err != nil {
    // level string was invalid
}
```

`log.Setup` initializes `github.com/projecteru2/core/log` from an environment
variable, defaulting to `info`. Every cocoonstack binary calls it once from
`main`; everything below uses `log.WithFunc("pkg.Func")` directly.
