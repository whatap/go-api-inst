# External Services Transformation Rules

## Redis

### github.com/gomodule/redigo

**Detection Pattern**: `redis.Dial()`, `redis.DialContext()`, `redis.DialURL()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo"
```

**Transformation Rule**:
```go
// Before
conn, err := redis.Dial("tcp", "localhost:6379")
conn, err := redis.DialContext(ctx, "tcp", "localhost:6379")

// After
conn, err := whatapredigo.Dial("tcp", "localhost:6379")
conn, err := whatapredigo.DialContext(ctx, "tcp", "localhost:6379")
```

---

### github.com/redis/go-redis (including v9)

**Detection Pattern**: `redis.NewClient()`, `redis.NewClusterClient()`, `redis.NewFailoverClient()`, `redis.NewRing()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis"
```

**Transformation Rule**:
```go
// Before
rdb := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// After
rdb := whatapgoredis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})
```

> **Note**: Only v8 (`github.com/go-redis/redis/v8`) and v9 (`github.com/redis/go-redis/v9`) are supported. v7 and earlier, v10+ are not supported and will be skipped.

---

## MongoDB

### go.mongodb.org/mongo-driver

**Detection Pattern**: `mongo.Connect()`, `mongo.NewClient()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo"
```

**Transformation Rule**:
```go
// Before
client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))

// After
client, err := whatapmongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
```

```go
// Before (deprecated NewClient)
client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))

// After
client, err := whatapmongo.NewClient(options.Client().ApplyURI(mongoURI))
```

> **Note**: whatapmongo automatically adds a CommandMonitor to track all MongoDB commands. If an existing Monitor exists, it is merged.

---

## Aerospike

### github.com/aerospike/aerospike-client-go (v6)

**Detection Pattern**: `aerospike.NewClient()`, `aerospike.NewClientWithPolicy()`, `client.Put()`, `client.Get()`, etc.

**Inserted Import**:
```go
import whatapdb "github.com/whatap/go-api/sql"
```

> **Note**: Aerospike doesn't support Hook/Middleware, so it's instrumented with the generic `whatapdb.Wrap*` functions.

**Transformation Rule (NewClient)**:
```go
// Before
client, err := aerospike.NewClient(host, port)

// After
client, err := whatapdb.WrapOpen(context.Background(), "aerospike", func() (*aerospike.Client, error) {
    return aerospike.NewClient(host, port)
})
```

**Transformation Rule (Put - error only)**:
```go
// Before
err := client.Put(policy, key, bins)

// After
err := whatapdb.WrapError(context.Background(), "aerospike", "Put", func() error {
    return client.Put(policy, key, bins)
})
```

**Transformation Rule (Get - returns value)**:
```go
// Before
record, err := client.Get(policy, key)

// After
record, err := whatapdb.Wrap(context.Background(), "aerospike", "Get", func() (*aerospike.Record, error) {
    return client.Get(policy, key)
})
```

**Supported Methods**:

| Method | Wrap Function | Return Type |
|--------|--------------|-------------|
| `Put`, `PutBins`, `Append`, `Prepend`, `Add`, `Touch`, `Close` | `WrapError` | `error` |
| `Get`, `GetHeader`, `Operate` | `Wrap` | `(*Record, error)` |
| `Exists`, `Delete` | `Wrap` | `(bool, error)` |
| `BatchGet`, `BatchGetHeader` | `Wrap` | `([]*Record, error)` |
| `Query`, `ScanAll`, `ScanNode` | `Wrap` | `(*Recordset, error)` |

> **Note**: Return types are inferred automatically. Only v6 and v8 are supported. v5 and earlier, v7, v9+ are not supported and will be skipped.

---

## Kafka

### github.com/IBM/sarama (or Shopify/sarama)

**Detection Pattern**: `sarama.NewSyncProducer()`, `sarama.NewAsyncProducer()`, `sarama.NewConsumer()`

**Inserted Import**:
```go
// When using IBM/sarama
import "github.com/whatap/go-api/instrumentation/github.com/IBM/sarama/whatapsarama"

// When using Shopify/sarama
import "github.com/whatap/go-api/instrumentation/github.com/Shopify/sarama/whatapsarama"
```

**Transformation Rule (ProducerInterceptor)**:
```go
// Before
config := sarama.NewConfig()
producer, err := sarama.NewSyncProducer(brokers, config)

// After
config := sarama.NewConfig()
config.Producer.Interceptors = []sarama.ProducerInterceptor{&whatapsarama.Interceptor{}}
producer, err := sarama.NewSyncProducer(brokers, config)
```

> **Note**: Both IBM/sarama and Shopify/sarama are supported. The correct whatapsarama is automatically selected based on the original import path.

### WrapConfig

As an alternative to direct Interceptor insertion, `WrapConfig` wraps the config instance:

```go
// Before
config := sarama.NewConfig()

// After
config := whatapsarama.WrapConfig(sarama.NewConfig())
```

**Signature**: `whatapsarama.WrapConfig(*sarama.Config) *sarama.Config`

> **Note**: `WrapConfig` internally sets `config.Producer.Interceptors` and returns the config. Used for struct field initialization and return statement patterns.

---

## gRPC

### google.golang.org/grpc

**Detection Pattern**: `grpc.NewServer()`, `grpc.Dial()`, `grpc.NewClient()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc"
```

**Transformation Rule (Server)**:
```go
// Before
s := grpc.NewServer()

// After
s := grpc.NewServer(
    grpc.ChainUnaryInterceptor(whatapgrpc.UnaryServerInterceptor()),
    grpc.ChainStreamInterceptor(whatapgrpc.StreamServerInterceptor()),
)
```

> **Why `Chain*`**: chaining options are additive, so the injected interceptor coexists with any interceptor the application already configured. The non-chaining `UnaryInterceptor` / `StreamInterceptor` options can be set only once and would panic at startup if the app already set them ("interceptor was already set and may not be reset").

**Transformation Rule (Client)**:
```go
// Before
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())

// After
conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(),
    grpc.WithChainUnaryInterceptor(whatapgrpc.UnaryClientInterceptor()),
    grpc.WithChainStreamInterceptor(whatapgrpc.StreamClientInterceptor()),
)
```

> **Note**: `grpc.Dial()`, `grpc.DialContext()`, and `grpc.NewClient()` are all supported.
>
> **Recommendation**: In gRPC v1.63.0+, `grpc.Dial()` and `grpc.DialContext()` are deprecated.
> Use `grpc.NewClient()` for new code.
> - Reference: [grpc-go #7090](https://github.com/grpc/grpc-go/issues/7090)

---

## Kubernetes

### k8s.io/client-go

**Detection Pattern**: `kubernetes.NewForConfig()`, `rest.InClusterConfig()`, `clientcmd.BuildConfigFromFlags()`

**Inserted Import**:
```go
import "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes"
```

**Transformation Rule**:
```go
// Before
config, err := rest.InClusterConfig()
if err != nil {
    config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
}
clientset, err := kubernetes.NewForConfig(config)

// After
config, err := rest.InClusterConfig()
if err != nil {
    config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
}
config.Wrap(whatapkubernetes.WrapRoundTripper())  // Inserted
clientset, err := kubernetes.NewForConfig(config)
```

> **Note**: `config.Wrap()` is automatically inserted just before the `kubernetes.NewForConfig()` call.

---

## Whatap Import Paths

| Original Package | Whatap Instrumentation Import |
|-----------------|------------------------------|
| `github.com/gomodule/redigo` | `.../gomodule/redigo/whatapredigo` |
| `github.com/redis/go-redis/v9` | `.../redis/go-redis/v9/whatapgoredis` |
| `go.mongodb.org/mongo-driver/mongo` | `.../go.mongodb.org/mongo-driver/mongo/whatapmongo` |
| `github.com/aerospike/aerospike-client-go` | `github.com/whatap/go-api/sql` (alias: whatapdb) |
| `github.com/IBM/sarama` | `.../IBM/sarama/whatapsarama` |
| `github.com/Shopify/sarama` | `.../Shopify/sarama/whatapsarama` |
| `google.golang.org/grpc` | `.../google.golang.org/grpc/whatapgrpc` |
| `k8s.io/client-go` | `.../k8s.io/client-go/kubernetes/whatapkubernetes` |

> **Note**: All paths are prefixed with `github.com/whatap/go-api/instrumentation/`
> **Exception**: Aerospike uses the generic `whatapdb.Wrap*` functions.
