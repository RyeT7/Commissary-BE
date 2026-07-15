package http

import (
	"time"

	"commissary/internal/domain/account"
	domainvideo "commissary/internal/domain/video"
)

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type channelResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Handle string `json:"handle"`
}

type sessionResponse struct {
	User    userResponse    `json:"user"`
	Channel channelResponse `json:"channel"`
}

type videoResponse struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	ChannelID     string `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ChannelHandle string `json:"channel_handle"`
	MIMEType      string `json:"mime_type"`
	Views         int64  `json:"views"`
	HasThumbnail  bool   `json:"has_thumbnail"`
	CreatedAt     string `json:"created_at"`
}

type channelPageDTO struct {
	Channel channelResponse `json:"channel"`
	Videos  []videoResponse `json:"videos"`
}

func sessionDTO(u *account.User, c *account.Channel) sessionResponse {
	return sessionResponse{User: userDTO(u), Channel: channelDTO(c)}
}

func userDTO(u *account.User) userResponse {
	return userResponse{ID: string(u.ID), Email: u.Email}
}

func channelDTO(c *account.Channel) channelResponse {
	if c == nil {
		return channelResponse{}
	}
	return channelResponse{ID: string(c.ID), Name: c.Name, Handle: c.Handle}
}

func (h *Handler) videoDTO(v *domainvideo.Video, c *account.Channel) videoResponse {
	dto := videoResponse{
		ID:           string(v.ID),
		Title:        v.Title,
		Description:  v.Description,
		ChannelID:    v.ChannelID,
		MIMEType:     v.MIMEType,
		Views:        v.Views,
		HasThumbnail: v.ThumbnailKey != "",
		CreatedAt:    v.CreatedAt.Format(time.RFC3339),
	}
	if c != nil {
		dto.ChannelName = c.Name
		dto.ChannelHandle = c.Handle
	}
	return dto
}
