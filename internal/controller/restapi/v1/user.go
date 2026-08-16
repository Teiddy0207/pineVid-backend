package v1

import (
	"errors"
	"net/http"

	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/request"
	"github.com/evrone/go-clean-template/internal/controller/restapi/v1/response"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/mapper"
	"github.com/gofiber/fiber/v2"
)


// @Summary     Register
// @Description Register a new user
// @ID          register
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request body     request.Register true "Registration data"
// @Success     201     {object} entity.User
// @Failure     400     {object} response.Error
// @Failure     409     {object} response.Error
// @Failure     500     {object} response.Error
// @Router      /v1/auth/register [post]
func (r *V1) register(ctx *fiber.Ctx) error {
	var body request.Register

	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - register")

		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		r.l.Error(err, "restapi - v1 - register")

		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	user, err := r.u.Register(ctx.UserContext(), body.Username, body.Email, body.Password)
	if err != nil {
		r.l.Error(err, "restapi - v1 - register")

		if errors.Is(err, entity.ErrUserAlreadyExists) {
			return errorResponse(ctx, http.StatusConflict, "user already exists")
		}

		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusCreated).JSON(user)
}

// @Summary     Login
// @Description Authenticate user and get JWT token
// @ID          login
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request body     request.Login true "Login credentials"
// @Success     200     {object} response.Token
// @Failure     400     {object} response.Error
// @Failure     401     {object} response.Error
// @Failure     500     {object} response.Error
// @Router      /v1/auth/login [post]
func (r *V1) login(ctx *fiber.Ctx) error {
	var body request.Login

	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - login")

		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		r.l.Error(err, "restapi - v1 - login")

		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	token, err := r.u.Login(ctx.UserContext(), body.Email, body.Password)
	if err != nil {
		r.l.Error(err, "restapi - v1 - login")

		if errors.Is(err, entity.ErrInvalidCredentials) {
			return errorResponse(ctx, http.StatusUnauthorized, "invalid credentials")
		}
		if errors.Is(err, entity.ErrUserBanned) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}

		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(response.Token{Token: token})
}

// @Summary     Refresh access token
// @Description Obtain a new JWT access token using a valid refresh token
// @ID          refreshToken
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request body     request.RefreshToken true "Refresh token payload"
// @Success     200     {object} response.Token
// @Failure     400     {object} response.Error
// @Failure     401     {object} response.Error
// @Failure     500     {object} response.Error
// @Router      /v1/auth/refresh [post]
func (r *V1) refreshToken(ctx *fiber.Ctx) error {
	var body request.RefreshToken

	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - refreshToken")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	if err := r.v.Struct(body); err != nil {
		r.l.Error(err, "restapi - v1 - refreshToken")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	tokenDTO, err := r.u.RefreshToken(ctx.UserContext(), body.RefreshToken)
	if err != nil {
		r.l.Error(err, "restapi - v1 - refreshToken")
		if errors.Is(err, entity.ErrInvalidCredentials) {
			return errorResponse(ctx, http.StatusUnauthorized, "invalid refresh token")
		}
		if errors.Is(err, entity.ErrUserBanned) {
			return errorResponse(ctx, http.StatusForbidden, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(tokenDTO)
}


// @Summary     Get profile
// @Description Get current user profile
// @ID          profile
// @Tags        Auth
// @Produce     json
// @Success     200 {object} entity.User
// @Failure     401 {object} response.Error
// @Failure     404 {object} response.Error
// @Failure     500 {object} response.Error
// @Security    BearerAuth
// @Router      /v1/user/profile [get]
func (r *V1) profile(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	user, err := r.u.GetUser(ctx.UserContext(), userID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - profile")

		if errors.Is(err, entity.ErrUserNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "user not found")
		}

		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(mapper.ToUserResponse(user))
}

// @Summary     Update profile
// @Description Update user profile details
// @ID          updateProfile
// @Tags        Auth
// @Accept      json
// @Produce     json
// @Param       request body     request.UpdateProfile true "Profile details"
// @Success     200     {object} entity.User
// @Failure     400     {object} response.Error
// @Failure     401     {object} response.Error
// @Failure     500     {object} response.Error
// @Security    BearerAuth
// @Router      /v1/user/profile [put]
func (r *V1) updateProfile(ctx *fiber.Ctx) error {
	userID, ok := ctx.Locals("userID").(string)
	if !ok {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	var body request.UpdateProfile
	if err := ctx.BodyParser(&body); err != nil {
		r.l.Error(err, "restapi - v1 - updateProfile")
		return errorResponse(ctx, http.StatusBadRequest, "invalid request body")
	}

	user, err := r.u.UpdateUser(ctx.UserContext(), userID, body.Username, body.Email, body.Avatar)
	if err != nil {
		r.l.Error(err, "restapi - v1 - updateProfile")

		if errors.Is(err, entity.ErrUserNotFound) {
			return errorResponse(ctx, http.StatusNotFound, "user not found")
		}

		return errorResponse(ctx, http.StatusInternalServerError, "internal server error")
	}

	return ctx.Status(http.StatusOK).JSON(mapper.ToUserResponse(user))
}

func (r *V1) getChannelDetails(ctx *fiber.Ctx) error {
	identifier := ctx.Params("id")
	if identifier == "" {
		return errorResponse(ctx, http.StatusBadRequest, "channel identifier required")
	}

	user, err := r.u.GetUser(ctx.UserContext(), identifier)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getChannelDetails")
		return errorResponse(ctx, http.StatusNotFound, "channel not found")
	}

	vids, err := r.vd.ListPublicVideos(ctx.UserContext(), user.ID, "", "", 1, 100)
	var totalViews int64 = 0
	total := 0
	if err == nil {
		total = vids.Pagination.TotalItems
		for _, v := range vids.Data {
			totalViews += int64(v.Views)
		}
	}

	subCount, err := r.fw.CountFollowers(ctx.UserContext(), user.ID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getChannelDetails - CountFollowers")
	}

	viewerID, _ := ctx.Locals("userID").(string)
	isFollowing, err := r.fw.IsFollowing(ctx.UserContext(), viewerID, user.ID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - getChannelDetails - IsFollowing")
	}

	return ctx.Status(http.StatusOK).JSON(response.ChannelDetailsResponse{
		ID:               user.ID,
		Username:         user.Username,
		Email:            user.Email,
		Avatar:           user.Avatar,
		Bio:              "Verified Content Creator on PipeVid Platform.",
		SubscribersCount: subCount,
		IsFollowing:      isFollowing,
		TotalVideos:      total,
		TotalViews:       totalViews,
		CreatedAt:        user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// @Summary      Follow/unfollow a channel
// @Description  Toggle following the given channel (user) for the current authenticated viewer
// @Tags         Channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Channel (user) ID"
// @Success      200 {object} response.FollowToggleResponse
// @Failure      400 {object} response.Error
// @Failure      401 {object} response.Error
// @Router       /v1/channels/{id}/follow [post]
func (r *V1) toggleFollowChannel(ctx *fiber.Ctx) error {
	channelID := ctx.Params("id")
	followerID, ok := ctx.Locals("userID").(string)
	if !ok || followerID == "" {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	resDTO, err := r.fw.ToggleFollow(ctx.UserContext(), followerID, channelID)
	if err != nil {
		r.l.Error(err, "restapi - v1 - toggleFollowChannel")
		if errors.Is(err, entity.ErrCannotFollowSelf) {
			return errorResponse(ctx, http.StatusBadRequest, err.Error())
		}
		return errorResponse(ctx, http.StatusInternalServerError, "failed to toggle follow")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}

// @Summary      List followed channels
// @Description  Paginated list of channels the current user follows
// @Tags         Channels
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Page limit" default(10)
// @Success      200 {object} response.PageResponse[response.ChannelSummary]
// @Failure      401 {object} response.Error
// @Router       /v1/user/following [get]
func (r *V1) listFollowedChannels(ctx *fiber.Ctx) error {
	followerID, ok := ctx.Locals("userID").(string)
	if !ok || followerID == "" {
		return errorResponse(ctx, http.StatusUnauthorized, "unauthorized")
	}

	page := ctx.QueryInt("page", 1)
	limit := ctx.QueryInt("limit", 10)
	if limit > 50 {
		limit = 50
	}

	resDTO, err := r.fw.ListFollowedChannels(ctx.UserContext(), followerID, page, limit)
	if err != nil {
		r.l.Error(err, "restapi - v1 - listFollowedChannels")
		return errorResponse(ctx, http.StatusInternalServerError, "failed to list followed channels")
	}

	return ctx.Status(http.StatusOK).JSON(resDTO)
}
