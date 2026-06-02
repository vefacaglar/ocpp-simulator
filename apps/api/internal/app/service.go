package app

import (
	"github.com/user/ocpp-simulator/apps/api/internal/db"
	"github.com/user/ocpp-simulator/apps/api/internal/simulator"
)

type Service struct {
	runtime *simulator.Runtime
	repo    *db.ChargePointRepo
}

func NewService(runtime *simulator.Runtime, repo *db.ChargePointRepo) *Service {
	return &Service{
		runtime: runtime,
		repo:    repo,
	}
}
