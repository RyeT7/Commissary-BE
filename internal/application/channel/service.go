package channel

import (
	"context"

	"commissary/internal/application/port"
	"commissary/internal/domain/account"
)

type Service struct {
	channels port.ChannelRepository
}

func NewService(channels port.ChannelRepository) *Service {
	return &Service{channels: channels}
}

func (s *Service) Get(ctx context.Context, id account.ChannelID) (*account.Channel, error) {
	return s.channels.FindByID(ctx, id)
}

func (s *Service) GetByUser(ctx context.Context, userID account.UserID) (*account.Channel, error) {
	return s.channels.FindByUserID(ctx, userID)
}
