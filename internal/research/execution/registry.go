package execution

import (
	"fmt"
	"sort"
	"sync"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

// Registry stores strategy adapters by their immutable code/version pair.
// List always returns a fresh, lexicographically ordered snapshot.
type Registry struct {
	mu         sync.RWMutex
	strategies map[protocolv2.StrategyRef]Strategy
}

func NewRegistry() *Registry {
	return &Registry{strategies: make(map[protocolv2.StrategyRef]Strategy)}
}

// Register adds one strategy. A code/version pair may only be registered once.
func (r *Registry) Register(strategy Strategy) error {
	if r == nil {
		return fmt.Errorf("execution: nil strategy registry")
	}
	if strategy == nil {
		return fmt.Errorf("execution: strategy is required")
	}
	metadata := strategy.Metadata()
	if err := metadata.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.strategies[metadata.Ref]; exists {
		return fmt.Errorf("execution: %s: %w", metadata.Ref, ErrDuplicateStrategy)
	}
	r.strategies[metadata.Ref] = strategy
	return nil
}

// Get retrieves a registered strategy by its code/version pair.
func (r *Registry) Get(ref protocolv2.StrategyRef) (Strategy, bool) {
	if r == nil {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	strategy, ok := r.strategies[ref]
	return strategy, ok
}

// List returns all metadata ordered by strategy code and then version.
func (r *Registry) List() []StrategyMetadata {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	metadata := make([]StrategyMetadata, 0, len(r.strategies))
	for _, strategy := range r.strategies {
		metadata = append(metadata, strategy.Metadata())
	}
	r.mu.RUnlock()

	sort.Slice(metadata, func(i, j int) bool {
		if metadata[i].Ref.Code == metadata[j].Ref.Code {
			return metadata[i].Ref.Version < metadata[j].Ref.Version
		}
		return metadata[i].Ref.Code < metadata[j].Ref.Code
	})
	return metadata
}

// ErrDuplicateStrategy allows callers to classify duplicate registration
// without parsing an error string.
var ErrDuplicateStrategy = fmt.Errorf("duplicate strategy registration")
