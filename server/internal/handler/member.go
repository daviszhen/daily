package handler

import (
	"net/http"
	"smart-daily/internal/model"
	"smart-daily/internal/repository"
	"strconv"

	"github.com/gin-gonic/gin"
)

type MemberHandler struct{ repo *repository.MemberRepo }

func NewMemberHandler(repo *repository.MemberRepo) *MemberHandler { return &MemberHandler{repo: repo} }

func (h *MemberHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	members, err := h.repo.ListActive(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	teamMap, _ := h.repo.TeamMap(ctx)

	type memberResp struct {
		model.Member
		TeamName string `json:"team_name"`
	}
	resp := make([]memberResp, len(members))
	for i, m := range members {
		resp[i] = memberResp{Member: m, TeamName: teamMap[m.TeamID]}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *MemberHandler) ListTeams(c *gin.Context) {
	teams, err := h.repo.ListTeams(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, teams)
}

func (h *MemberHandler) CreateTeam(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	team := model.Team{Name: req.Name}
	if err := h.repo.CreateTeam(c.Request.Context(), &team); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "team already exists"})
		return
	}
	c.JSON(http.StatusOK, team)
}

func (h *MemberHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.repo.SoftDelete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *MemberHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Status string `json:"status"`
		Team   string `json:"team"`
		TeamID *int   `json:"team_id"`
		Role   string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	updates := map[string]interface{}{}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Team != "" {
		updates["team"] = req.Team
	}
	if req.TeamID != nil {
		updates["team_id"] = *req.TeamID
	}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nothing to update"})
		return
	}
	if err := h.repo.Update(c.Request.Context(), id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
