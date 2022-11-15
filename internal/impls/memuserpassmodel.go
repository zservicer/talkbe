package impls

import (
	"context"
	"sync"
	"time"

	"github.com/godruoyi/go-snowflake"
	"github.com/sbasestarter/bizinters/userinters/userpass"
	"github.com/sgostarter/i/commerr"
)

func NewMemUserPassModel() userpass.UserPasswordModel {
	return &memUserPassModelImpl{
		users: make(map[uint64]*userpass.User),
	}
}

type memUserPassModelImpl struct {
	usersLock sync.Mutex
	users     map[uint64]*userpass.User
}

func (impl *memUserPassModelImpl) AddUser(ctx context.Context, userName, password string) (user *userpass.User, err error) {
	impl.usersLock.Lock()
	defer impl.usersLock.Unlock()

	for _, u := range impl.users {
		if u.UserName == userName {
			err = commerr.ErrAlreadyExists

			return
		}
	}

	uid := snowflake.ID()

	impl.users[uid] = &userpass.User{
		ID:       uid,
		UserName: userName,
		Password: password,
		CreateAt: time.Now().Unix(),
	}

	user = &(*impl.users[uid])

	return
}

func (impl *memUserPassModelImpl) DeleteUser(ctx context.Context, userID uint64) error {
	impl.usersLock.Lock()
	defer impl.usersLock.Unlock()

	delete(impl.users, userID)

	return nil
}

func (impl *memUserPassModelImpl) GetUser(ctx context.Context, userID uint64) (user *userpass.User, err error) {
	impl.usersLock.Lock()
	defer impl.usersLock.Unlock()

	u, ok := impl.users[userID]
	if !ok {
		err = commerr.ErrNotFound

		return
	}

	user = &userpass.User{
		ID:       u.ID,
		UserName: u.UserName,
		Password: u.Password,
		CreateAt: u.CreateAt,
	}

	return
}

func (impl *memUserPassModelImpl) GetUserByUserName(ctx context.Context, userName string) (user *userpass.User, err error) {
	impl.usersLock.Lock()
	defer impl.usersLock.Unlock()

	for _, u := range impl.users {
		if u.UserName == userName {
			user = &userpass.User{
				ID:       u.ID,
				UserName: u.UserName,
				Password: u.Password,
				CreateAt: u.CreateAt,
			}

			return
		}
	}

	err = commerr.ErrNotFound

	return
}

func (impl *memUserPassModelImpl) ListUsers(ctx context.Context) (users []*userpass.User, err error) {
	impl.usersLock.Lock()
	defer impl.usersLock.Unlock()

	for _, u := range impl.users {
		users = append(users, &userpass.User{
			ID:       u.ID,
			UserName: u.UserName,
			Password: u.Password,
			CreateAt: u.CreateAt,
		})
	}

	return
}
