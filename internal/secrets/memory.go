package secrets

// Memory is an in-memory Provider for tests.
type Memory struct {
	values map[string]string
}

func NewMemory() *Memory {
	return &Memory{values: map[string]string{}}
}

func memoryKey(profile, name string) string {
	return profile + "\x00" + name
}

func (m *Memory) Get(profile, name string) (string, error) {
	value, ok := m.values[memoryKey(profile, name)]
	if !ok {
		return "", ErrNotFound
	}
	return value, nil
}

func (m *Memory) Set(profile, name, value string) error {
	m.values[memoryKey(profile, name)] = value
	return nil
}

func (m *Memory) Delete(profile, name string) error {
	key := memoryKey(profile, name)
	if _, ok := m.values[key]; !ok {
		return ErrNotFound
	}
	delete(m.values, key)
	return nil
}
