package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/gofiber/fiber/v2"
)

// @Summary      Set preferred categories
// @Description  Replace the caller's preferred video categories (used to seed cold-start personalized recommendations)
// @Tags         User
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body request.SetPreferencesRequest true "Preferred Categories"
// @Success      200 {object} map[string]bool
// @Failure      400 {object} response.Error
// @Router       /v1/user/preferences [put]
func (r *V1) setUserPreferences(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

	var body request.SetPreferencesRequest
	if err := ctx.BodyParser(&body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}
	if err := r.v.Struct(body); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, err.Error())
	}

	if err := r.up.SetPreferredCategories(ctx.UserContext(), userID, body.Categories); err != nil {
		r.l.Error(err, "restapi - v1 - setUserPreferences")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to save preferences")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"success": true})
}

// @Summary      Get preferred categories
// @Description  List the caller's preferred video categories
// @Tags         User
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Router       /v1/user/preferences [get]
func (r *V1) getUserPreferences(ctx *fiber.Ctx) error {
	userID := getUserID(ctx)

	categories, err := r.up.GetPreferredCategories(ctx.UserContext(), userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getUserPreferences")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to load preferences")
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{"categories": categories})
}
