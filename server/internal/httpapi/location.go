package httpapi

import (
	"errors"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"safecircle/internal/store"
)

func (s *Server) setSharing(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	user := currentUser(c)
	if err := s.store.SetSharing(c.Request.Context(), user.ID, req.Enabled, s.Now()); err != nil {
		s.fail(c, err)
		return
	}
	if s.live != nil {
		if err := s.live.PublishSharing(c.Request.Context(), user.ID, req.Enabled); err != nil {
			log.Printf("redis sharing: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

func (s *Server) postLocation(c *gin.Context) {
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	if !validCoord(req.Latitude, req.Longitude) || math.IsNaN(req.AccuracyM) || req.AccuracyM < 0 || req.AccuracyM > 100000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That location could not be read."})
		return
	}
	user := currentUser(c)
	stored, loc, err := s.store.AddLocation(c.Request.Context(), user.ID, req.Latitude, req.Longitude, req.AccuracyM, s.Now())
	if err != nil {
		s.fail(c, err)
		return
	}
	body := gin.H{"stored": stored, "live": false}
	if !stored {
		c.JSON(http.StatusOK, body)
		return
	}
	if s.live != nil {
		if err := s.live.PublishLocation(c.Request.Context(), user.ID, loc.Latitude, loc.Longitude, loc.AccuracyM, loc.RecordedAt); err != nil {
			log.Printf("redis location: %v", err)
		} else {
			body["live"] = true
		}
	}
	body["location"] = s.locJSON(loc)
	c.JSON(http.StatusOK, body)
}

func (s *Server) listPlaces(c *gin.Context) {
	user := currentUser(c)
	places, err := s.store.Places(c.Request.Context(), user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"places": placesJSON(places)})
}

func (s *Server) createPlace(c *gin.Context) {
	var req struct {
		Label     string  `json:"label"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" || len([]rune(label)) > 40 || !validCoord(req.Latitude, req.Longitude) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name the place and include a location."})
		return
	}
	user := currentUser(c)
	place, err := s.store.CreatePlace(c.Request.Context(), user.ID, label, req.Latitude, req.Longitude, s.Now())
	if err != nil {
		if errors.Is(err, store.ErrCap) {
			c.JSON(http.StatusConflict, gin.H{"error": "You can save up to 20 places."})
			return
		}
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, placeJSON{ID: place.ID, Label: place.Label, Latitude: place.Latitude, Longitude: place.Longitude})
}

func (s *Server) deletePlace(c *gin.Context) {
	user := currentUser(c)
	if err := s.store.DeletePlace(c.Request.Context(), user.ID, c.Param("placeId")); err != nil {
		s.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) trail(c *gin.Context) {
	user := currentUser(c)
	personID := c.Param("personId")
	if ok, err := s.store.IsActiveSupervisor(c.Request.Context(), personID, user.ID); err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found."})
		return
	}
	since := s.Now().Add(-24 * time.Hour)
	points, err := s.store.Trail(c.Request.Context(), personID, since)
	if err != nil {
		s.fail(c, err)
		return
	}
	out := make([]locJSON, 0, len(points))
	for _, p := range points {
		out = append(out, s.locJSON(p))
	}
	c.JSON(http.StatusOK, gin.H{"trail": out})
}

func (s *Server) personPlaces(c *gin.Context) {
	user := currentUser(c)
	personID := c.Param("personId")
	if ok, err := s.store.IsActiveSupervisor(c.Request.Context(), personID, user.ID); err != nil || !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found."})
		return
	}
	places, err := s.store.Places(c.Request.Context(), personID)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"places": placesJSON(places)})
}

func validCoord(lat, lng float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lng) && !math.IsInf(lat, 0) && !math.IsInf(lng, 0) &&
		lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}
