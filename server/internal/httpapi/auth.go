package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"safecircle/internal/age"
	"safecircle/internal/store"
	"safecircle/internal/token"
)

type credentials struct {
	DisplayName string `json:"displayName"`
	Passphrase  string `json:"passphrase"`
}

type userJSON struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IDOnFile    bool   `json:"idOnFile"`
}

func (s *Server) register(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	name := strings.TrimSpace(req.DisplayName)
	if !validDisplayName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a name of 1 to 40 characters."})
		return
	}
	if len(req.Passphrase) < 8 || len(req.Passphrase) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a passphrase of 8 to 72 characters."})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Passphrase), s.bcryptCost)
	if err != nil {
		s.fail(c, err)
		return
	}
	id := uuid.NewString()
	if err := s.store.CreateUser(c.Request.Context(), id, name, strings.ToLower(name), string(hash), s.Now()); err != nil {
		if errors.Is(err, store.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "That name is already in use."})
			return
		}
		s.fail(c, err)
		return
	}
	s.writeSession(c, id)
}

func (s *Server) login(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That request could not be read."})
		return
	}
	user, err := s.store.UserByNameKey(c.Request.Context(), strings.ToLower(strings.TrimSpace(req.DisplayName)))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PassphraseHash), []byte(req.Passphrase)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Name or passphrase is wrong."})
		return
	}
	s.writeSession(c, user.ID)
}

func (s *Server) logout(c *gin.Context) {
	raw := bearer(c)
	if err := s.store.DeleteSession(c.Request.Context(), token.Hash(raw)); err != nil {
		s.fail(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Server) writeSession(c *gin.Context, userID string) {
	now := s.Now()
	raw, hash, err := token.New()
	if err != nil {
		s.fail(c, err)
		return
	}
	if err := s.store.CreateSession(c.Request.Context(), hash, userID, now, now.Add(tokenTTL)); err != nil {
		s.fail(c, err)
		return
	}
	user, err := s.store.UserByToken(c.Request.Context(), hash, now)
	if err != nil {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": raw, "user": userJSON{ID: user.ID, DisplayName: user.DisplayName, IDOnFile: user.IDOnFile}})
}

func (s *Server) me(c *gin.Context) {
	user := currentUser(c)
	ctx := c.Request.Context()
	circle, err := s.store.Circle(ctx, user.ID)
	if err != nil {
		s.fail(c, err)
		return
	}
	body := gin.H{
		"id":                user.ID,
		"displayName":       user.DisplayName,
		"idOnFile":          user.IDOnFile,
		"supervisorLimit":   circle.SupervisorLimit,
		"activeSupervisors": circle.ActiveSupervisors,
		"sharingEnabled":    circle.SharingEnabled,
	}
	if user.IDOnFile {
		idn, err := s.store.Identity(ctx, user.ID)
		if err != nil {
			s.fail(c, err)
			return
		}
		body["identity"] = gin.H{
			"legalName":     idn.LegalName,
			"dateOfBirth":   idn.DateOfBirth.Format("2006-01-02"),
			"documentType":  idn.DocumentType,
			"documentLast4": idn.DocumentLast4,
			"status":        idn.Status,
		}
	}
	if loc, err := s.store.LatestLocation(ctx, user.ID); err == nil {
		body["lastLocation"] = s.locJSON(loc)
	} else if !errors.Is(err, store.ErrNotFound) {
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, body)
}

func (s *Server) submitID(c *gin.Context) {
	user := currentUser(c)
	if user.IDOnFile {
		c.JSON(http.StatusConflict, gin.H{"error": "An ID is already on file."})
		return
	}
	legal := strings.TrimSpace(c.PostForm("legalName"))
	dobRaw := strings.TrimSpace(c.PostForm("dateOfBirth"))
	docType := strings.TrimSpace(c.PostForm("documentType"))
	docNum := strings.TrimSpace(c.PostForm("documentNumber"))
	if legal == "" || len([]rune(legal)) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the name printed on the ID."})
		return
	}
	dob, err := time.Parse("2006-01-02", dobRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the date of birth as YYYY-MM-DD."})
		return
	}
	if !validDocType(docType) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Choose a passport, national ID, or driver's license."})
		return
	}
	if !validDocNumber(docNum) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Enter the document number from the ID."})
		return
	}
	if !age.Over16(dob, s.Now()) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Circle is for people over 16."})
		return
	}
	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Add a photo of your ID."})
		return
	}
	opened, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Add a photo of your ID."})
		return
	}
	defer opened.Close()
	data, err := io.ReadAll(io.LimitReader(opened, 8<<20+1))
	if err != nil {
		s.fail(c, err)
		return
	}
	if len(data) > 8<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "That photo is too large."})
		return
	}
	ext, ok := sniffImage(data)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use a JPEG, PNG, or WebP photo."})
		return
	}
	if err := os.MkdirAll(s.uploadDir, 0o700); err != nil {
		s.fail(c, err)
		return
	}
	photoPath := filepath.Join(s.uploadDir, user.ID+ext)
	if err := os.WriteFile(photoPath, data, 0o600); err != nil {
		s.fail(c, err)
		return
	}
	sum := sha256.Sum256([]byte(strings.ToUpper(docNum)))
	numberHash := hex.EncodeToString(sum[:])
	last4 := docNum
	if len(docNum) > 4 {
		last4 = docNum[len(docNum)-4:]
	}
	if err := s.store.SaveIdentity(c.Request.Context(), user.ID, legal, docType, numberHash, last4, photoPath, dob, s.Now()); err != nil {
		_ = os.Remove(photoPath)
		if errors.Is(err, store.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "That ID is already on file."})
			return
		}
		s.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"idOnFile": true,
		"identity": gin.H{
			"legalName":     legal,
			"dateOfBirth":   dob.Format("2006-01-02"),
			"documentType":  docType,
			"documentLast4": last4,
			"status":        "id_on_file",
		},
	})
}

func validDisplayName(name string) bool {
	if name == "" || len([]rune(name)) > 40 {
		return false
	}
	for _, r := range name {
		if r == '\n' || r == '\r' {
			return false
		}
	}
	return true
}

func validDocType(t string) bool {
	switch t {
	case "passport", "national_id", "drivers_license":
		return true
	default:
		return false
	}
}

func validDocNumber(s string) bool {
	if len(s) < 4 || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func sniffImage(b []byte) (string, bool) {
	if bytes.HasPrefix(b, []byte{0x89, 'P', 'N', 'G'}) {
		return ".png", true
	}
	if bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}) {
		return ".jpg", true
	}
	if len(b) > 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return ".webp", true
	}
	return "", false
}
