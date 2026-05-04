// Package users in v2 path collides on short name with v1/users.
package users

// User is the v2 flavor.
type User struct {
	Name string `json:"name"`
}
