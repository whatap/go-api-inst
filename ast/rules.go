package ast

// AllRules returns all rules (Tier 1: 37 + Phase 2: 8 + Phase 3a: 15 + Phase 3b: 6 + Phase 3c: 26 = 92).
func AllRules() []*Rule {
	return []*Rule{
		// ── ReplaceFunction (25) ──────────────────────────────────────

		// sql (1)
		{Target: "database/sql.Open", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/database/sql/whatapsql", WhatapAlias: "whatapsql", WhatapFunc: "Open",
		}},

		// sqlx (5)
		{Target: "github.com/jmoiron/sqlx.Open", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx", WhatapAlias: "whatapsqlx", WhatapFunc: "Open",
		}},
		{Target: "github.com/jmoiron/sqlx.Connect", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx", WhatapAlias: "whatapsqlx", WhatapFunc: "Connect",
		}},
		{Target: "github.com/jmoiron/sqlx.ConnectContext", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx", WhatapAlias: "whatapsqlx", WhatapFunc: "ConnectContext",
		}},
		{Target: "github.com/jmoiron/sqlx.MustConnect", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx", WhatapAlias: "whatapsqlx", WhatapFunc: "MustConnect",
		}},
		{Target: "github.com/jmoiron/sqlx.MustOpen", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jmoiron/sqlx/whatapsqlx", WhatapAlias: "whatapsqlx", WhatapFunc: "MustOpen",
		}},

		// gorm (1)
		{Target: "gorm.io/gorm.Open", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-gorm/gorm/whatapgorm", WhatapAlias: "whatapgorm", WhatapFunc: "Open",
		}},

		// jinzhugorm (1)
		{Target: "github.com/jinzhu/gorm.Open", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/jinzhu/gorm/whatapgorm", WhatapAlias: "whatapgorm", WhatapFunc: "Open",
		}},

		// goredis v9 (4)
		{Target: "github.com/redis/go-redis/v9.NewClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewClient",
		}},
		{Target: "github.com/redis/go-redis/v9.NewClusterClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewClusterClient",
		}},
		{Target: "github.com/redis/go-redis/v9.NewFailoverClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewFailoverClient",
		}},
		{Target: "github.com/redis/go-redis/v9.NewRing", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/redis/go-redis/v9/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewRing",
		}},

		// goredis v8 (4)
		{Target: "github.com/go-redis/redis/v8.NewClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewClient",
		}},
		{Target: "github.com/go-redis/redis/v8.NewClusterClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewClusterClient",
		}},
		{Target: "github.com/go-redis/redis/v8.NewFailoverClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewFailoverClient",
		}},
		{Target: "github.com/go-redis/redis/v8.NewRing", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-redis/redis/v8/whatapgoredis", WhatapAlias: "whatapgoredis", WhatapFunc: "NewRing",
		}},

		// redigo (4)
		{Target: "github.com/gomodule/redigo/redis.Dial", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo", WhatapAlias: "whatapredigo", WhatapFunc: "Dial",
		}},
		{Target: "github.com/gomodule/redigo/redis.DialContext", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo", WhatapAlias: "whatapredigo", WhatapFunc: "DialContext",
		}},
		{Target: "github.com/gomodule/redigo/redis.DialURL", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo", WhatapAlias: "whatapredigo", WhatapFunc: "DialURL",
		}},
		{Target: "github.com/gomodule/redigo/redis.DialURLContext", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gomodule/redigo/whatapredigo", WhatapAlias: "whatapredigo", WhatapFunc: "DialURLContext",
		}},

		// mongo (2)
		{Target: "go.mongodb.org/mongo-driver/mongo.Connect", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo", WhatapAlias: "whatapmongo", WhatapFunc: "Connect",
		}},
		{Target: "go.mongodb.org/mongo-driver/mongo.NewClient", Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/go.mongodb.org/mongo-driver/mongo/whatapmongo", WhatapAlias: "whatapmongo", WhatapFunc: "NewClient",
		}},

		// fmt (3) — §242: OptIn. High-frequency log apps (Loki, Promtail) pay
		// +30~43% p99 from whatapfmt forwarding, so fmt is disabled by default.
		// Users who want fmt logs in the WhaTap logsink add `enabled_packages: [fmt]`.
		{Target: "fmt.Print", OptIn: true, Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/fmt/whatapfmt", WhatapAlias: "whatapfmt", WhatapFunc: "Print",
		}},
		{Target: "fmt.Printf", OptIn: true, Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/fmt/whatapfmt", WhatapAlias: "whatapfmt", WhatapFunc: "Printf",
		}},
		{Target: "fmt.Println", OptIn: true, Advice: &ReplaceFunction{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/fmt/whatapfmt", WhatapAlias: "whatapfmt", WhatapFunc: "Println",
		}},

		// ── WrapCall (12) ─────────────────────────────────────────────

		// gin (2)
		{Target: "github.com/gin-gonic/gin.Default", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin", WhatapAlias: "whatapgin", WhatapFunc: "WrapEngine",
		}},
		{Target: "github.com/gin-gonic/gin.New", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gin-gonic/gin/whatapgin", WhatapAlias: "whatapgin", WhatapFunc: "WrapEngine",
		}},

		// echo v4 (1)
		{Target: "github.com/labstack/echo/v4.New", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/v4/whatapecho", WhatapAlias: "whatapecho", WhatapFunc: "WrapEcho",
		}},
		// echo v3 (1)
		{Target: "github.com/labstack/echo.New", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/labstack/echo/whatapecho", WhatapAlias: "whatapecho", WhatapFunc: "WrapEcho",
		}},

		// fiber (1)
		{Target: "github.com/gofiber/fiber/v2.New", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gofiber/fiber/v2/whatapfiber", WhatapAlias: "whatapfiber", WhatapFunc: "WrapApp",
		}},

		// chi (2) — v4 and v5 share the same whatap package
		{Target: "github.com/go-chi/chi.NewRouter", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi", WhatapAlias: "whatapchi", WhatapFunc: "WrapRouter",
		}},
		{Target: "github.com/go-chi/chi/v5.NewRouter", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/go-chi/chi/whatapchi", WhatapAlias: "whatapchi", WhatapFunc: "WrapRouter",
		}},

		// gorilla (2)
		{Target: "github.com/gorilla/mux.NewRouter", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux", WhatapAlias: "whatapmux", WhatapFunc: "WrapRouter",
		}},
		{Target: "github.com/gorilla/mux.Route.Subrouter", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/gorilla/mux/whatapmux", WhatapAlias: "whatapmux", WhatapFunc: "WrapRouter",
		}},

		// sarama (2)
		{Target: "github.com/IBM/sarama.NewConfig", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/IBM/sarama/whatapsarama", WhatapAlias: "whatapsarama", WhatapFunc: "WrapConfig",
		}},
		{Target: "github.com/Shopify/sarama.NewConfig", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/Shopify/sarama/whatapsarama", WhatapAlias: "whatapsarama", WhatapFunc: "WrapConfig",
		}},

		// logrus (1)
		{Target: "github.com/sirupsen/logrus.New", Advice: &WrapCall{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/sirupsen/logrus/whataplogrus", WhatapAlias: "whataplogrus", WhatapFunc: "WrapLogger",
		}},

		// ── Phase 2: ArgInsert + CodeInsert + MainInsert + ArgWrap ─────────

		// grpc — ArgInsert (4)
		{Target: "google.golang.org/grpc.NewServer", Advice: &ArgInsert{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc", WhatapAlias: "whatapgrpc",
			InsertArgs: []InsertedArg{
				// §294: Chain* variants are additive — they coexist with a user's
				// existing grpc.UnaryInterceptor/StreamInterceptor instead of panicking
				// ("the unary server interceptor was already set"). Plain options are single-slot.
				{WrapFunc: "ChainUnaryInterceptor", InnerFunc: "UnaryServerInterceptor"},
				{WrapFunc: "ChainStreamInterceptor", InnerFunc: "StreamServerInterceptor"},
			},
			Ellipsis: true,
		}, Signature: &FuncSignature{MinArgs: 0, MaxArgs: -1}},
		{Target: "google.golang.org/grpc.Dial", Advice: &ArgInsert{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc", WhatapAlias: "whatapgrpc",
			InsertArgs: []InsertedArg{
				{WrapFunc: "WithChainUnaryInterceptor", InnerFunc: "UnaryClientInterceptor"},
				{WrapFunc: "WithChainStreamInterceptor", InnerFunc: "StreamClientInterceptor"},
			},
			Ellipsis: true,
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: -1}},
		{Target: "google.golang.org/grpc.DialContext", Advice: &ArgInsert{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc", WhatapAlias: "whatapgrpc",
			InsertArgs: []InsertedArg{
				{WrapFunc: "WithChainUnaryInterceptor", InnerFunc: "UnaryClientInterceptor"},
				{WrapFunc: "WithChainStreamInterceptor", InnerFunc: "StreamClientInterceptor"},
			},
			Ellipsis: true,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: -1}},
		{Target: "google.golang.org/grpc.NewClient", Advice: &ArgInsert{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/google.golang.org/grpc/whatapgrpc", WhatapAlias: "whatapgrpc",
			InsertArgs: []InsertedArg{
				{WrapFunc: "WithChainUnaryInterceptor", InnerFunc: "UnaryClientInterceptor"},
				{WrapFunc: "WithChainStreamInterceptor", InnerFunc: "StreamClientInterceptor"},
			},
			Ellipsis: true,
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: -1}},

		// k8s — CodeInsert (2)
		{Target: "k8s.io/client-go/kubernetes.NewForConfig", Advice: &CodeInsert{
			WhatapPkg:   "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes",
			WhatapAlias: "whatapkubernetes", Position: "before",
			ArgSource: 0, MethodName: "Wrap", WhatapFunc: "WrapRoundTripper",
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: 1}},
		{Target: "k8s.io/client-go/kubernetes.NewForConfigOrDie", Advice: &CodeInsert{
			WhatapPkg:   "github.com/whatap/go-api/instrumentation/k8s.io/client-go/kubernetes/whatapkubernetes",
			WhatapAlias: "whatapkubernetes", Position: "before",
			ArgSource: 0, MethodName: "Wrap", WhatapFunc: "WrapRoundTripper",
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: 1}},

		// zap — 보류 (§63 TraceLogWriter 방식으로 전환 예정, HookStderr 폐기)

		// log — ArgWrap (1) + MainInsert (1)
		{Target: "log.New", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/logsink", WhatapAlias: "whataplogsink",
			WhatapFunc: "GetTraceLogWriter", ArgIndex: 0,
		}, Signature: &FuncSignature{MinArgs: 3, MaxArgs: 3}},
		{Target: "log.SetOutput", Advice: &MainInsert{
			WhatapPkg: "github.com/whatap/go-api/logsink", WhatapAlias: "whataplogsink",
			WhatapFunc: "GetTraceLogWriter", ExtraImport: "os", WrapExpr: "os.Stderr",
			OrigPkgAlias: "log", OrigFunc: "SetOutput",
		}},

		// ── Phase 3b: nethttp ReplaceWithCtx ──────────────────────────────

		// nethttp — ReplaceWithCtx (3): package-level client functions
		{Target: "net/http.Get", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "HttpGet", OrigFunc: "Get",
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: 1}},
		{Target: "net/http.Post", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "HttpPost", OrigFunc: "Post",
		}, Signature: &FuncSignature{MinArgs: 3, MaxArgs: 3}},
		{Target: "net/http.PostForm", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "HttpPostForm", OrigFunc: "PostForm",
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// nethttp — ReplaceWithCtx (3): DefaultClient methods
		{Target: "net/http.DefaultClient.Get", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "DefaultClientGet", OrigVar: "DefaultClient", OrigFunc: "Get",
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: 1}},
		{Target: "net/http.DefaultClient.Post", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "DefaultClientPost", OrigVar: "DefaultClient", OrigFunc: "Post",
		}, Signature: &FuncSignature{MinArgs: 3, MaxArgs: 3}},
		{Target: "net/http.DefaultClient.PostForm", Advice: &ReplaceWithCtx{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "DefaultClientPostForm", OrigVar: "DefaultClient", OrigFunc: "PostForm",
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// ── Phase 3a: nethttp ArgWrap + FieldWrap + FieldInsert ───────────

		// nethttp — ArgWrap (4): handler wrapping
		{Target: "net/http.HandleFunc", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "net/http.Handle", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "WrapHandler", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "net/http.ServeMux.HandleFunc", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "net/http.ServeMux.Handle", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "WrapHandler", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// nethttp — FieldWrap (1): Server{Handler}
		{Target: "net/http.Server{}", Advice: &FieldWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WhatapFunc: "WrapHandler", FieldName: "Handler",
		}, Fields: []FieldMatch{{Name: "Handler", Required: true}}},

		// nethttp — FieldWrap+FieldInsert (1): Client{} Transport handling
		// Wraps existing Transport or inserts new one — both cases in one Rule.
		{Target: "net/http.Client{}", Advice: &FieldWrapOrInsert{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/net/http/whataphttp", WhatapAlias: "whataphttp",
			WrapFunc:   "NewRoundTrip",                   // when Transport exists
			InsertFunc: "NewRoundTripWithEmptyTransport", // when Transport missing
			FieldName:  "Transport",
			CtxAware:   true,
		}},

		// ── Phase 3a: fasthttp ArgWrap + FieldWrap ────────────────────────

		// fasthttp — ArgWrap (8): router method handler wrapping
		{Target: "github.com/fasthttp/router.Router.GET", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.POST", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.PUT", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.DELETE", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.PATCH", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.HEAD", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.OPTIONS", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/fasthttp/router.Router.ANY", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "Func", ArgIndex: -1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// fasthttp — FieldWrap (1): Server{Handler}
		{Target: "github.com/valyala/fasthttp.Server{}", Advice: &FieldWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/github.com/valyala/fasthttp/whatapfasthttp", WhatapAlias: "whatapfasthttp",
			WhatapFunc: "WrapHandler", FieldName: "Handler",
		}, Fields: []FieldMatch{{Name: "Handler", Required: true}}},

		// ── Phase 3c: Transform — aerospike (26 Rule) ─────────────────────

		// aerospike — WrapOpen (3): NewClient, NewClientWithPolicy, NewClientWithPolicyAndHost
		{Target: "github.com/aerospike/aerospike-client-go/v6.NewClient", Advice: &Transform{
			Template:      `whatapdb.WrapOpen({{.Ctx}}, fmt.Sprintf("aerospike://%v:%v", {{.Arg0}}, {{.Arg1}}), func() (*{{.TargetPkg}}.Client, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "context", "fmt"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.NewClientWithPolicy", Advice: &Transform{
			Template:      `whatapdb.WrapOpen({{.Ctx}}, fmt.Sprintf("aerospike://%v:%v", {{.Arg1}}, {{.Arg2}}), func() (*{{.TargetPkg}}.Client, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "context", "fmt"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		// §236 — variadic *Host. {{.Args1Plus}} preserves the original call's
		// spread/individual form so the substituted call still compiles. The
		// whatapas.DbhostFromHosts helper accepts variadic *Host and produces
		// the dbhost string regardless of cardinality.
		{Target: "github.com/aerospike/aerospike-client-go/v6.NewClientWithPolicyAndHost", Advice: &Transform{
			Template: `whatapdb.WrapOpen({{.Ctx}}, whatapas.DbhostFromHosts({{.Args1Plus}}), func() (*{{.TargetPkg}}.Client, error) { return {{.Original}} })`,
			Imports: []string{
				"github.com/whatap/go-api/sql",
				"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas",
				"context",
			},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},

		// aerospike — WrapPut (1)
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Put", Advice: &Transform{
			Template: `whatapas.WrapPut({{.Ctx}}, {{.Receiver}}, {{.Arg1}}, {{.Arg2}}, func() error { return {{.Original}} })`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
		}},

		// aerospike — WrapPutBins (1)
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.PutBins", Advice: &Transform{
			Template: `whatapas.WrapPutBins({{.Ctx}}, {{.Receiver}}, {{.Arg1}}, func() error { return {{.Original}} })`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
		}},

		// aerospike — WrapGet (1)
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Get", Advice: &Transform{
			Template: `whatapas.WrapGet({{.Ctx}}, {{.Receiver}}, {{.Arg1}}, nil, func() (*{{.TargetPkg}}.Record, error) { return {{.Original}} })`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
		}},

		// aerospike — WrapDelete (1)
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Delete", Advice: &Transform{
			Template: `whatapas.WrapDelete({{.Ctx}}, {{.Receiver}}, {{.Arg1}}, func() (bool, error) { return {{.Original}} })`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
		}},

		// aerospike — WrapExists (1)
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Exists", Advice: &Transform{
			Template: `whatapas.WrapExists({{.Ctx}}, {{.Receiver}}, {{.Arg1}}, func() (bool, error) { return {{.Original}} })`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
		}},

		// aerospike — WrapError (7): error-only methods
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Append", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Prepend", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Add", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Touch", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Truncate", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.CreateIndex", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.DropIndex", Advice: &Transform{
			Template:      `whatapdb.WrapError({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() error { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},

		// aerospike — Wrap (11): (T, error) methods — hardcoded return types from v1 aerospikeMethodReturnTypes
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.GetHeader", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Record, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.BatchGet", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() ([]*{{.TargetPkg}}.Record, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.BatchGetHeader", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() ([]*{{.TargetPkg}}.Record, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.BatchExists", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() ([]bool, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.BatchDelete", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() ([]*{{.TargetPkg}}.BatchRecord, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Query", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Recordset, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.ScanAll", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Recordset, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.ScanNode", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Recordset, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Operate", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Record, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.Execute", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (interface{}, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},
		{Target: "github.com/aerospike/aerospike-client-go/v6.Client.QueryAggregate", Advice: &Transform{
			Template:      `whatapdb.Wrap({{.Ctx}}, whatapas.GetDbhost({{.Receiver}}), "{{.FuncName}}", func() (*{{.TargetPkg}}.Recordset, error) { return {{.Original}} })`,
			Imports:       []string{"github.com/whatap/go-api/sql", "github.com/whatap/go-api/instrumentation/github.com/aerospike/aerospike-client-go/v6/whatapas", "context"},
			ImportAliases: map[string]string{"github.com/whatap/go-api/sql": "whatapdb"},
		}},

		// §254 — sashabaranov/go-openai 메서드 wrap (Transform): user
		// code's `c.CreateChatCompletion(ctx, req)` → wrap helper call so
		// the *openai.Client variable type stays unchanged. The helpers
		// route through the same code path as manual WrapClient(c).Method.
		{Target: "github.com/sashabaranov/go-openai.Client.CreateChatCompletion", Advice: &Transform{
			Template: `whatapopenai.WrapAndCreateChatCompletion({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},
		{Target: "github.com/sashabaranov/go-openai.Client.CreateChatCompletionStream", Advice: &Transform{
			Template: `whatapopenai.WrapAndCreateChatCompletionStream({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},
		{Target: "github.com/sashabaranov/go-openai.Client.CreateCompletion", Advice: &Transform{
			Template: `whatapopenai.WrapAndCreateCompletion({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},
		{Target: "github.com/sashabaranov/go-openai.Client.CreateEmbeddings", Advice: &Transform{
			Template: `whatapopenai.WrapAndCreateEmbeddings({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},

		// §254 — sashabaranov/go-openai constructor wrap (Transform): user
		// code's `openai.NewClient(key)` / `openai.NewClientWithConfig(cfg)`
		// becomes a whatapopenai helper that installs a wrapped RoundTripper
		// on cfg.HTTPClient. Returned *openai.Client type unchanged so user
		// variable declarations stay valid.
		{Target: "github.com/sashabaranov/go-openai.NewClient", Advice: &Transform{
			Template: `whatapopenai.NewClient({{.Arg0}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},
		{Target: "github.com/sashabaranov/go-openai.NewClientWithConfig", Advice: &Transform{
			Template: `whatapopenai.NewClientFromConfig({{.Arg0}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/sashabaranov/go-openai/whatapopenai"},
		}},

		// §254 — cloudwego/eino-ext openai/claude constructor wrap
		// (Transform): user's `einoopenai.NewChatModel(ctx, cfg)` becomes
		// `whatapeino.NewOpenAIChatModel(ctx, cfg)` which copies cfg and
		// installs a wrapped RoundTripper on cfg.HTTPClient. Returned
		// concrete *einoopenai.ChatModel / *einoclaude.ChatModel preserves
		// user variable type. WrapChatModel for response-metadata extraction
		// stays user-explicit (interface-wrap not auto-injected — §3.2).
		{Target: "github.com/cloudwego/eino-ext/components/model/openai.NewChatModel", Advice: &Transform{
			Template: `whatapeino.NewOpenAIChatModel({{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}},
		{Target: "github.com/cloudwego/eino-ext/components/model/claude.NewChatModel", Advice: &Transform{
			Template: `whatapeino.NewClaudeChatModel({{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}},

		// §282 작업1 — cloudwego/eino compose call-site wrap (ArgWrap):
		// the model argument passed into a compose builder is wrapped so the
		// graph runtime invokes the WhaTap decorator's Generate/Stream and
		// extracts response metadata (model/token/op). The §254 constructor
		// rule only captures the HTTP round-trip (provider via URL); compose
		// ArgWrap adds the response-metadata extraction without the user
		// writing WrapChatModel by hand.
		//
		// All five methods take the model as `model.BaseChatModel`, so the wrap
		// helper WrapBaseChatModel accepts/returns model.BaseChatModel —
		// the argument's static type is unchanged, so this can never introduce
		// a compile error (AST safety). The helper dispatches on the inner's
		// dynamic type to keep ToolCallingChatModel/ChatModel capabilities for
		// eino's internal assertions. Idempotent.
		//
		// Generic receivers (Chain[I,O] / Workflow[I,O]) match via §282 작업0
		// (resolveMethodTarget strips type args). Graph[I,O] embeds *graph;
		// callers hold *Graph[I,O], so the receiver type resolves to Graph.
		//
		// ArgIndex: AppendChatModel(node, opts...) → 0;
		// the others take (key, node, opts...) → 1. opts is variadic (MaxArgs -1).
		{Target: "github.com/cloudwego/eino/compose.Chain.AppendChatModel", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino", WhatapAlias: "whatapeino",
			WhatapFunc: "WrapBaseChatModel", ArgIndex: 0,
		}, Signature: &FuncSignature{MinArgs: 1, MaxArgs: -1}},
		{Target: "github.com/cloudwego/eino/compose.Graph.AddChatModelNode", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino", WhatapAlias: "whatapeino",
			WhatapFunc: "WrapBaseChatModel", ArgIndex: 1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: -1}},
		{Target: "github.com/cloudwego/eino/compose.Workflow.AddChatModelNode", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino", WhatapAlias: "whatapeino",
			WhatapFunc: "WrapBaseChatModel", ArgIndex: 1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: -1}},
		{Target: "github.com/cloudwego/eino/compose.ChainBranch.AddChatModel", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino", WhatapAlias: "whatapeino",
			WhatapFunc: "WrapBaseChatModel", ArgIndex: 1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: -1}},
		{Target: "github.com/cloudwego/eino/compose.Parallel.AddChatModel", Advice: &ArgWrap{
			WhatapPkg: "github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino", WhatapAlias: "whatapeino",
			WhatapFunc: "WrapBaseChatModel", ArgIndex: 1,
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: -1}},

		// §282 작업2 — cloudwego/eino-ext direct call wrap (Transform):
		// a direct `cm.Generate(ctx, in)` / `cm.Stream(ctx, in)` on the concrete
		// *einoopenai.ChatModel / *einoclaude.ChatModel (the type returned by the
		// §254 constructor replacement) is rewritten to the call-site helper
		// `whatapeino.WrapGenerate(cm, ctx, in)` so the WhaTap LLM step (response
		// metadata) is emitted. cm stays its concrete type — the helper borrows
		// it as model.BaseChatModel, so no user type changes (AST safety).
		//
		// Signature{MaxArgs:2}: only the canonical (ctx, input) form is rewritten;
		// when call options are supplied the call is left unchanged (manual
		// WrapGenerate / WrapBaseChatModel covers that). Interface-typed cm
		// (model.ChatModel var) is the §228 interface-matching case — not here.
		{Target: "github.com/cloudwego/eino-ext/components/model/openai.ChatModel.Generate", Advice: &Transform{
			Template: `whatapeino.WrapGenerate({{.Receiver}}, {{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/cloudwego/eino-ext/components/model/openai.ChatModel.Stream", Advice: &Transform{
			Template: `whatapeino.WrapStream({{.Receiver}}, {{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/cloudwego/eino-ext/components/model/claude.ChatModel.Generate", Advice: &Transform{
			Template: `whatapeino.WrapGenerate({{.Receiver}}, {{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/cloudwego/eino-ext/components/model/claude.ChatModel.Stream", Advice: &Transform{
			Template: `whatapeino.WrapStream({{.Receiver}}, {{.Arg0}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/cloudwego/eino/whatapeino"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// §253 — anthropics/anthropic-sdk-go MessageService wrap (Transform):
		// user code's `client.Messages.New(ctx, params)` →
		// `whatapanthropic.WrapAndNewMessage(ctx, client.Messages, params)`.
		// Receiver = `client.Messages` (MessageService value) is forwarded
		// directly — the wrap helper takes a MessageService so the 2-step
		// selector pattern maps to a free function without any Client wrapper
		// struct or Engine extension.
		//
		// Signature{MaxArgs:2}: SDK accepts trailing opts but auto-inject
		// only handles the canonical `c.Messages.New(ctx, params)` form to
		// avoid silently dropping user-supplied option arguments. Manual
		// adapter use (`whatapanthropic.WrapAndNewMessage`) covers the opts
		// case.
		{Target: "github.com/anthropics/anthropic-sdk-go.MessageService.New", Advice: &Transform{
			Template: `whatapanthropic.WrapAndNewMessage({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/anthropics/anthropic-sdk-go/whatapanthropic"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/anthropics/anthropic-sdk-go.MessageService.NewStreaming", Advice: &Transform{
			Template: `whatapanthropic.WrapAndNewMessageStreaming({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/anthropics/anthropic-sdk-go/whatapanthropic"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// §253 — anthropic.NewClient(opts ...option.RequestOption) constructor
		// wrap. Whatap helper prepends an option.WithHTTPClient(wrapped)
		// before user opts (Anthropic SDK applies options last-write-wins, so
		// user-supplied HTTPClient overrides the wrap). Returned anthropic.Client
		// value type is preserved — user variable declarations stay valid.
		//
		// Variadic note: `{{.Args}}` forwards all literal arguments. The
		// `opts...` spread form is a known limitation — `nodeToString`
		// drops the trailing `...`, so users invoking spread should use
		// manual wrap (`whatapanthropic.NewClient(opts...)`) or rely on
		// non-spread call sites (the common pattern in the Anthropic SDK
		// examples).
		{Target: "github.com/anthropics/anthropic-sdk-go.NewClient", Advice: &Transform{
			Template: `whatapanthropic.NewClient({{.Args}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/anthropics/anthropic-sdk-go/whatapanthropic"},
		}},

		// §255 — openai/openai-go (공식 SDK) ChatCompletionService wrap (Transform):
		// user code's `client.Chat.Completions.New(ctx, params)` →
		// `whatapopenaigo.WrapAndNewChatCompletion(ctx, client.Chat.Completions, params)`.
		// 3단계 selector chain (Client.Chat.Completions.New) — Receiver template
		// 이 `client.Chat.Completions` 그대로 전달, wrap helper 가
		// ChatCompletionService 값을 받음 (§253 패턴 복제).
		//
		// Signature{MaxArgs:2}: SDK accepts trailing opts but auto-inject
		// only handles the canonical `s.New(ctx, params)` form to avoid
		// silently dropping user-supplied option arguments. Manual adapter
		// (`whatapopenaigo.WrapAndNewChatCompletion`) covers the opts case.
		{Target: "github.com/openai/openai-go.ChatCompletionService.New", Advice: &Transform{
			Template: `whatapopenaigo.WrapAndNewChatCompletion({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/openai/openai-go/whatapopenaigo"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},
		{Target: "github.com/openai/openai-go.ChatCompletionService.NewStreaming", Advice: &Transform{
			Template: `whatapopenaigo.WrapAndNewChatCompletionStreaming({{.Arg0}}, {{.Receiver}}, {{.Arg1}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/openai/openai-go/whatapopenaigo"},
		}, Signature: &FuncSignature{MinArgs: 2, MaxArgs: 2}},

		// §255 — openai.NewClient(opts ...option.RequestOption) constructor
		// wrap. Whatap helper prepends option.WithHTTPClient(wrapped) before
		// user opts (OpenAI Go SDK applies options last-write-wins, so user-
		// supplied HTTPClient overrides the wrap). {{.Args}} forwards all
		// literal arguments — the `opts...` spread form is a known
		// limitation (nodeToString drops the trailing `...`); manual wrap
		// covers it.
		{Target: "github.com/openai/openai-go.NewClient", Advice: &Transform{
			Template: `whatapopenaigo.NewClient({{.Args}})`,
			Imports:  []string{"github.com/whatap/go-api/instrumentation/llm/github.com/openai/openai-go/whatapopenaigo"},
		}},
	}
}
