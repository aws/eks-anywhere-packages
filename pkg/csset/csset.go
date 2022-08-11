package csset

import (
	"strings"
	"sync"
)

// CSSet is a comma-separated set of strings.
type CSSet struct {
	mu  sync.Mutex
	set map[string]struct{}
}

func NewCSSet(s string) *CSSet {
	c := &CSSet{
		set: make(map[string]struct{}),
	}
	if s == "" {
		return c
	}

	for _, value := range strings.Split(s, ",") {
		c.Add(value)
	}
	return c
}

func (c *CSSet) Add(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set[value] = struct{}{}
}

func (c *CSSet) Del(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.set, value)
}

func (c *CSSet) Has(value string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, found := c.set[value]
	return found
}

func (c *CSSet) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.set)
}

func (c *CSSet) String() string {
	out := []string{}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.set {
		out = append(out, key)
	}
	return strings.Join(out, ",")
}
