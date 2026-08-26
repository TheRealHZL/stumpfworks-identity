package directory

import "context"

type User struct{ Username, DisplayName, DN, Mail string }
type Directory interface {
	UserExists(context.Context, string) (bool, error)
	GetUser(context.Context, string) (*User, error)
	ListUsers(context.Context) ([]User, error)
	AuthenticateUser(context.Context, string, string) (*User, error)
	AuthenticateAdmin(context.Context, string, string) (*User, error)
}
type Local struct{}

func (Local) UserExists(context.Context, string) (bool, error) { return true, nil }
func (Local) GetUser(_ context.Context, u string) (*User, error) {
	return &User{Username: u, DisplayName: u}, nil
}
func (Local) ListUsers(context.Context) ([]User, error) { return nil, nil }
func (Local) AuthenticateUser(_ context.Context, u, _ string) (*User, error) {
	return &User{Username: u, DisplayName: u}, nil
}
func (Local) AuthenticateAdmin(_ context.Context, u, _ string) (*User, error) {
	return &User{Username: u, DisplayName: u}, nil
}
