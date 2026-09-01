package v1

import (
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/gofiber/fiber/v2"
)

func (r *V1) saveWord(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	var req request.SaveWordRequest
	if err := ctx.BodyParser(&req); err != nil {
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	res, err := r.vocab.SaveWord(ctx.UserContext(), userID, req)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	return ctx.Status(http.StatusCreated).JSON(response.Response[response.VocabularyResponse]{
		Success: true,
		Message: "Word saved to vocabulary notebook",
		Data:    res,
	})
}

func (r *V1) listWords(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	res, err := r.vocab.ListWords(ctx.UserContext(), userID)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[[]response.VocabularyResponse]{
		Success: true,
		Data:    res,
	})
}

func (r *V1) deleteWord(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	id := ctx.Params("id")
	if err := r.vocab.DeleteWord(ctx.UserContext(), id, userID); err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[bool]{
		Success: true,
		Message: "Word deleted successfully",
		Data:    true,
	})
}

func (r *V1) lookupDictionary(ctx *fiber.Ctx) error {
	word := ctx.Query("word")
	if word == "" {
		return errorResponse(ctx, http.StatusBadRequest, "word query parameter required")
	}

	res, err := r.vocab.LookupWord(ctx.UserContext(), word)
	if err != nil {
		return errorResponse(ctx, http.StatusInternalServerError, err.Error())
	}

	return ctx.Status(http.StatusOK).JSON(response.Response[response.DictionaryLookupResponse]{
		Success: true,
		Data:    res,
	})
}
