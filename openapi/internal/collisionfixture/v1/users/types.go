// Package users in v1 path collides on short name with v2/users.
package users

// User is the v1 flavor.
type User struct {
	ID int64 `json:"id"`
}
