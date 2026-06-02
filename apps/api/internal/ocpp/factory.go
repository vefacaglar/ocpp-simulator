package ocpp

type Factory struct {
	protocols map[string]Protocol
}

func NewFactory() *Factory {
	return &Factory{
		protocols: make(map[string]Protocol),
	}
}

func (f *Factory) Register(p Protocol) {
	f.protocols[p.Version()] = p
}

func (f *Factory) Create(version string) (Protocol, bool) {
	p, ok := f.protocols[version]
	return p, ok
}
