package main

import (
	"net/http"
	"os"
	"strconv"

	"github.com/fox-gonic/fox"
	"github.com/fox-gonic/fox/httperrors"
	"github.com/fox-gonic/fox/openapi"
)

type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ListUsersRequest struct {
	Page     int `query:"page" binding:"omitempty,gte=1"`
	PageSize int `query:"page_size" binding:"omitempty,gte=1,lte=100"`
}

type GetUserRequest struct {
	ID int64 `uri:"id" binding:"required,gt=0"`
}

type CreateUserRequest struct {
	Name  string `json:"name" binding:"required,min=3,max=80"`
	Email string `json:"email" binding:"required,email"`
}

type ListUsersResponse struct {
	Items []User `json:"items"`
	Page  int    `json:"page"`
	Total int    `json:"total"`
}

var users = []User{
	{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com"},
	{ID: 2, Name: "Grace Hopper", Email: "grace@example.com"},
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := fox.Default()

	router.GET("/users", listUsers)
	router.GET("/users/:id", getUser)
	router.POST("/users", createUser)

	spec := openapi.New(router,
		openapi.Info("Fox OpenAPI Example", "1.0.0"),
		openapi.Server("http://localhost:"+port),
	)

	router.GET("/openapi.yaml", openapi.YAMLHandler(spec))
	router.GET("/openapi.json", openapi.JSONHandler(spec))

	router.Run(":" + port)
}

func listUsers(_ *fox.Context, req ListUsersRequest) ListUsersResponse {
	page := req.Page
	if page == 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20
	}

	return ListUsersResponse{
		Items: users,
		Page:  page,
		Total: len(users),
	}
}

func getUser(_ *fox.Context, req GetUserRequest) (User, error) {
	for _, user := range users {
		if user.ID == req.ID {
			return user, nil
		}
	}

	return User{}, httperrors.New(http.StatusNotFound, "user "+strconv.FormatInt(req.ID, 10)+" not found").
		SetCode("USER_NOT_FOUND")
}

func createUser(_ *fox.Context, req CreateUserRequest) (User, error) {
	user := User{
		ID:    int64(len(users) + 1),
		Name:  req.Name,
		Email: req.Email,
	}
	users = append(users, user)
	return user, nil
}
