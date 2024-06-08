package service

import "sync"

// UserStore is an interface to store users.
type UserStore interface {
    // Save saves a user to the store.
    Save(user *User) error
    // Find finds a user by username.
    Find(username string) (*User, error)
}

type InMemoryUserStore struct {
    mu sync.RWMutex
    users map[string]*User
}

func NewInMemoryUserStore() *InMemoryUserStore {
    return &InMemoryUserStore{
        users: make(map[string]*User),
    }
}

// Save saves a user to the store.
func (store *InMemoryUserStore) Save(user *User) error {
    store.mu.Lock()
    defer store.mu.Unlock()

    if store.users[user.Username] != nil {
        return ErrAlreadyExists
    }

    store.users[user.Username] = user
    return nil
}

// Find finds a user by username.
func (store *InMemoryUserStore) Find(username string) (*User, error) {
    store.mu.RLock()
    defer store.mu.RUnlock()

    if store.users[username] == nil {
        return nil, ErrNotFound
    }

    return store.users[username], nil
}
