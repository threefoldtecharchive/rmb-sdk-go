package direct

import (
	"fmt"

	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/threefoldtech/substrate-client"
)

// TwinDB is used to get Twin instances
type TwinDB interface {
	GetTwin(id uint32) (Twin, error)
}

// Twin is used to store a twin id and its public key
type Twin struct {
	id        uint32
	publikKey []byte
}

type twinDBImpl struct {
	cache   *cache.Cache
	manager substrate.Manager
}

// NewTwinDB creates a new twinDBImpl instance, with a non expiring cache.
func NewTwinDB(manager substrate.Manager) TwinDB {
	return &twinDBImpl{
		cache:   cache.New(0, 0),
		manager: manager,
	}
}

// GetTwin gets Twin from cache if present. if not, gets it from substrate client and caches it.
func (t *twinDBImpl) GetTwin(id uint32) (Twin, error) {
	cachedValue, ok := t.cache.Get(fmt.Sprint(id))
	if ok {
		return cachedValue.(Twin), nil
	}

	sub, err := t.manager.Substrate()
	if err != nil {
		return Twin{}, err
	}
	defer sub.Close()

	substrateTwin, err := sub.GetTwin(id)
	if err != nil {
		return Twin{}, errors.Wrapf(err, "could net get twin with id %d", id)
	}

	twin := Twin{
		id:        id,
		publikKey: substrateTwin.Account.PublicKey(),
	}

	err = t.cache.Add(fmt.Sprint(id), twin, 0)
	if err != nil {
		return Twin{}, errors.Wrapf(err, "could not set cache for twin with id %d", id)
	}

	return twin, nil
}
