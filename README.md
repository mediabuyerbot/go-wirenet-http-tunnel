# go-wirenet-http-tunnel

```
wirenet client or server <--------> wirenet handler <---------------> http server
                          tunnel                       http request

```

## Examples
tunnel.go
```go
// server side
wire, err := wirenet.Mount(":8989")
// OR client side
wire, err := wirenet.Join(":8989")

...
tunnel := httptunnel.New()
wire.Stream("tunnel", tunnel.Handle)
```

client.go OR server.go
```go
 // server side
 wire, err := wirenet.Mount(":8989")
 // OR client side
 wire, err := wirenet.Join(":8989")
 ...

 // Make client
 client := httptunnel.NewClient(wire, "tunnel")
 ctx := context.Background()
 
 // 1. One request in one stream
 session := hub.findSession("id")
 sessCtx := WithSession(ctx, session.ID())
 req, err := http.NewRequestWithContext(sessCtx, "GET", "http://google.com", nil) 
 resp, err := client.Do(req)
 ...
 
 // 2. Group related queries in one stream
 err = client.WithTx(ctx, sid, func(streamCtx context.Context) error {
    req, err := http.NewRequestWithContext(sessCtx, "GET", "http://domain.com/1", nil) 
    resp, err := client.Do(req)	
    // ...
    req, err = http.NewRequestWithContext(sessCtx, "GET", "http://domain.com/2", nil) 
    resp, err = client.Do(req)	
    // ... 
    req, err = http.NewRequestWithContext(sessCtx, "GET", "http://domain.com/2", nil) 
    resp, err = client.Do(req)	
    return nil
})
 

```

