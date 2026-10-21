package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"safecircle/internal/checkflow"
	"safecircle/internal/store"
)

func (s *Server) watching(c *gin.Context) {
	s.expireAndPublish(c.Request.Context())
	user := currentUser(c)
	people, err := s.store.Watching(c.Request.Context(), user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(people))
	for _, person := range people {
		item := gin.H{
			"personId":       person.PersonID,
			"displayName":    person.DisplayName,
			"sharingEnabled": person.SharingEnabled,
		}
		if person.LastLocation != nil {
			item["lastLocation"] = s.locJSON(*person.LastLocation)
		}
		if person.Check != nil {
			item["check"] = toCheckJSON(*person.Check)
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"people": out})
}

func (s *Server) person(c *gin.Context) {
	s.expireAndPublish(c.Request.Context())
	user := currentUser(c)
	personID := c.Param("personId")
	ctx := c.Request.Context()
	if ok, err := s.store.IsActiveSupervisor(ctx, personID, user.ID); err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found."})
		return
	}
	people, err := s.store.Watching(ctx, user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	var found *store.Watched
	for i := range people {
		if people[i].PersonID == personID {
			found = &people[i]
			break
		}
	}
	if found == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found."})
		return
	}
	since := s.Now().Add(-24 * time.Hour)
	trail, err := s.store.Trail(ctx, personID, since)
	if err != nil {
		s.fail(c, err)
		return
	}
	places, err := s.store.Places(ctx, personID)
	if err != nil {
		s.fail(c, err)
		return
	}
	points := make([]locJSON, 0, len(trail))
	for _, p := range trail {
		points = append(points, s.locJSON(p))
	}
	body := gin.H{
		"personId":       found.PersonID,
		"displayName":    found.DisplayName,
		"sharingEnabled": found.SharingEnabled,
		"trail":          points,
		"places":         placesJSON(places),
	}
	if found.LastLocation != nil {
		body["lastLocation"] = s.locJSON(*found.LastLocation)
	}
	if found.Check != nil {
		body["check"] = toCheckJSON(*found.Check)
	}
	c.JSON(http.StatusOK, body)
}

func (s *Server) startCheck(c *gin.Context) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !checkflow.ValidReason(req.Reason) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a reason."})
		return
	}
	s.expireAndPublish(c.Request.Context())
	user := currentUser(c)
	chk, err := s.store.CreateCheck(c.Request.Context(), c.Param("personId"), user.ID, req.Reason, s.Now())
	if err != nil {
		s.fail(c, err)
		return
	}
	s.publishCheck(c.Request.Context(), chk)
	c.JSON(http.StatusOK, gin.H{"check": toCheckJSON(chk)})
}

func (s *Server) pendingCheck(c *gin.Context) {
	s.expireAndPublish(c.Request.Context())
	user := currentUser(c)
	chk, err := s.store.PendingCheck(c.Request.Context(), user.ID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusOK, gin.H{"check": nil})
		return
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"check": toCheckJSON(chk)})
}

func (s *Server) confirmCheck(c *gin.Context) {
	user := currentUser(c)
	chk, err := s.store.ConfirmCheck(c.Request.Context(), c.Param("checkId"), user.ID, s.Now())
	if errors.Is(err, store.ErrWindow) {
		if expired, getErr := s.store.GetCheck(c.Request.Context(), c.Param("checkId")); getErr == nil {
			s.publishCheck(c.Request.Context(), expired)
		}
		s.fail(c, err)
		return
	}
	if err != nil {
		s.fail(c, err)
		return
	}
	s.publishCheck(c.Request.Context(), chk)
	c.JSON(http.StatusOK, gin.H{"check": toCheckJSON(chk)})
}
