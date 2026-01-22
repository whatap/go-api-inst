package ast

import (
	"go-api-inst/ast/common"

	"github.com/dave/dst"

	// Register transformer packages (execute init())
	// Phase 2: Web frameworks
	_ "go-api-inst/ast/packages/chi"
	_ "go-api-inst/ast/packages/echo"
	_ "go-api-inst/ast/packages/fasthttp"
	_ "go-api-inst/ast/packages/fiber"
	_ "go-api-inst/ast/packages/gin"
	_ "go-api-inst/ast/packages/gorilla"
	_ "go-api-inst/ast/packages/nethttp"
	// Phase 3: Databases
	_ "go-api-inst/ast/packages/gorm"
	_ "go-api-inst/ast/packages/jinzhugorm"
	_ "go-api-inst/ast/packages/sql"
	_ "go-api-inst/ast/packages/sqlx"
	// Phase 4: External services
	_ "go-api-inst/ast/packages/aerospike"
	_ "go-api-inst/ast/packages/goredis"
	// Phase 7: Log libraries
	_ "go-api-inst/ast/packages/fmt"
	_ "go-api-inst/ast/packages/log"
	_ "go-api-inst/ast/packages/logrus"
	_ "go-api-inst/ast/packages/zap"
	_ "go-api-inst/ast/packages/grpc"
	_ "go-api-inst/ast/packages/k8s"
	_ "go-api-inst/ast/packages/mongo"
	_ "go-api-inst/ast/packages/redigo"
	_ "go-api-inst/ast/packages/sarama"
)

// Transformer is the per-package transformer interface
// Each package (gin, echo, sql, etc.) implements this interface to inject/remove instrumentation code.
// Note: This type is an alias for common.Transformer.
type Transformer = common.Transformer

// Register registers a transformer to the registry
// Called from each package's init() function.
func Register(t Transformer) {
	common.Register(t)
}

// GetTransformer retrieves a transformer by name
func GetTransformer(name string) Transformer {
	return common.GetTransformer(name)
}

// GetAllTransformers retrieves all registered transformers
func GetAllTransformers() []Transformer {
	return common.GetAllTransformers()
}

// GetDetectedTransformers retrieves transformers for packages detected in file
func GetDetectedTransformers(file *dst.File) []Transformer {
	return common.GetDetectedTransformers(file)
}

// GetTransformerNames retrieves all registered transformer names
func GetTransformerNames() []string {
	return common.GetTransformerNames()
}

// HasTransformer checks if a transformer is registered
func HasTransformer(name string) bool {
	return common.HasTransformer(name)
}

// ClearRegistry clears the registry (for testing)
func ClearRegistry() {
	common.ClearRegistry()
}

// GetEnabledTransformers retrieves transformers for enabled packages only
// If enabledPackages is nil, returns all transformers (backward compatibility)
func GetEnabledTransformers(enabledPackages []string) []Transformer {
	return common.GetEnabledTransformers(enabledPackages)
}

// GetFilteredTransformers retrieves enabled transformers from packages detected in file
// If enabledPackages is nil, returns all detected transformers (backward compatibility)
func GetFilteredTransformers(file *dst.File, enabledPackages []string) []Transformer {
	return common.GetFilteredTransformers(file, enabledPackages)
}
