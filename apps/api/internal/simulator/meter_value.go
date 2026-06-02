package simulator

import (
	"sync"
	"time"
)

type MeterValueGenerator struct {
	mu            sync.Mutex
	nominalPowerW int
	startMeterWh  int
	startTime     time.Time
	running       bool
}

func NewMeterValueGenerator(nominalPowerW int) *MeterValueGenerator {
	if nominalPowerW == 0 {
		nominalPowerW = 11000
	}
	return &MeterValueGenerator{nominalPowerW: nominalPowerW}
}

func (g *MeterValueGenerator) Start(startMeterWh int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.startMeterWh = startMeterWh
	g.startTime = time.Now()
	g.running = true
}

func (g *MeterValueGenerator) Stop() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running = false
	return g.currentMeterWh()
}

func (g *MeterValueGenerator) CurrentMeterWh() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.currentMeterWh()
}

func (g *MeterValueGenerator) currentMeterWh() int {
	if !g.running {
		return g.startMeterWh
	}
	elapsed := time.Since(g.startTime).Seconds()
	deltaWh := int(float64(g.nominalPowerW) * elapsed / 3600.0)
	return g.startMeterWh + deltaWh
}

func (g *MeterValueGenerator) IsRunning() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running
}
