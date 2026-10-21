package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"safecircle/internal/store"
	"safecircle/internal/token"
)

func (s *Server) setLimit(c *gin.Context) {
	var req struct {
		SupervisorLimit int `json:"supervisorLimit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	if req.SupervisorLimit < 1 || req.SupervisorLimit > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose between 1 and 5 supervisors."})
		return
	}
	user := currentUser(c)
	if err := s.store.SetLimit(c.Request.Context(), user.ID, req.SupervisorLimit, s.Now()); err != nil {
		s.fail(c, err)
		return
	}
	circle, err := s.store.Circle(c.Request.Context(), user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"supervisorLimit":   circle.SupervisorLimit,
		"activeSupervisors": circle.ActiveSupervisors,
	})
}

func (s *Server) createInvite(c *gin.Context) {
	user := currentUser(c)
	var code string
	var err error
	for i := 0; i < 5; i++ {
		code, err = token.Code(8)
		if err != nil {
			s.fail(c, err)
			return
		}
		err = s.store.CreateInvite(c.Request.Context(), user.ID, code, s.Now())
		if err == nil {
			c.JSON(http.StatusOK, gin.H{"code": code})
			return
		}
		if err == store.ErrConflict {
			continue
		}
		s.fail(c, err)
		return
	}
	s.fail(c, err)
}

func (s *Server) listSupervisors(c *gin.Context) {
	user := currentUser(c)
	list, err := s.store.Supervisors(c.Request.Context(), user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, sup := range list {
		out = append(out, gin.H{"linkId": sup.LinkID, "userId": sup.UserID, "displayName": sup.DisplayName})
	}
	c.JSON(http.StatusOK, gin.H{"supervisors": out})
}

func (s *Server) revoke(c *gin.Context) {
	user := currentUser(c)
	if err := s.store.Revoke(c.Request.Context(), user.ID, c.Param("linkId"), s.Now()); err != nil {
		s.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) acceptInvite(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Code) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter an invite code."})
		return
	}
	user := currentUser(c)
	personID, err := s.store.AcceptInvite(c.Request.Context(), user.ID, req.Code, s.Now())
	if err != nil {
		if err == store.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "That invite code is not valid."})
			return
		}
		if err == store.ErrConflict {
			c.JSON(http.StatusConflict, gin.H{"error": "You already supervise this person."})
			return
		}
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"personId": personID})
}
