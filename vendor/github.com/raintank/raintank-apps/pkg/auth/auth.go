package auth

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type int64SliceFlag []int64

func (i *int64SliceFlag) Set(value string) error {
	for _, split := range strings.Split(value, ",") {
		if split == "" {
			continue
		}
		parsed, err := strconv.Atoi(split)
		if err != nil {
			return err
		}
		*i = append(*i, int64(parsed))
	}
	return nil
}

func (i *int64SliceFlag) String() string {
	return strings.Trim(strings.Replace(fmt.Sprint(*i), " ", ", ", -1), "[]")
}

var (
	validTTL     = time.Minute * 5
	invalidTTL   = time.Second * 30
	authEndpoint = "https://grafana.net"
	validOrgIds  = int64SliceFlag{}
	cache        *AuthCache

	Debug = false

	// global HTTP client.  By sharing the client we can take
	// advantage of keepalives and re-use connections instead
	// of establishing a new tcp connection for every request.
	client = &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          10,
			IdleConnTimeout:       300 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
)

type AuthCache struct {
	sync.RWMutex
	items map[string]CacheItem
}

type CacheItem struct {
	User       *SignedInUser
	ExpireTime time.Time
}

func (a *AuthCache) Get(key string) (*SignedInUser, bool) {
	a.RLock()
	defer a.RUnlock()
	if c, ok := a.items[key]; ok {
		return c.User, c.ExpireTime.After(time.Now())
	}
	return nil, false
}

func (a *AuthCache) Set(key string, u *SignedInUser, ttl time.Duration) {
	a.Lock()
	a.items[key] = CacheItem{
		User:       u,
		ExpireTime: time.Now().Add(ttl),
	}
	a.Unlock()
}

func (a *AuthCache) Clear() {
	a.Lock()
	a.items = make(map[string]CacheItem)
	a.Unlock()
}

func init() {
	flag.StringVar(&authEndpoint, "auth-endpoint", authEndpoint, "Endpoint to authenticate users on")
	flag.DurationVar(&validTTL, "auth-valid-ttl", validTTL, "how long valid auth responses should be cached")
	flag.DurationVar(&invalidTTL, "auth-invalid-ttl", invalidTTL, "how long invalid auth responses should be cached")
	flag.Var(&validOrgIds, "auth-valid-org-id", "restrict authentication to the listed orgId (comma separated list)")
	cache = &AuthCache{items: make(map[string]CacheItem)}
}

func Auth(adminKey, keyString string) (*SignedInUser, error) {
	if keyString == adminKey {
		return &SignedInUser{
			Role:    ROLE_ADMIN,
			OrgId:   1,
			OrgName: "Admin",
			OrgSlug: "admin",
			IsAdmin: true,
			key:     keyString,
		}, nil
	}

	// check the cache
	if Debug {
		log.Println("Auth: Checking cache for apiKey")
	}
	user, cached := cache.Get(keyString)
	if cached {
		if user != nil {
			if Debug {
				log.Println("Auth: valid key cached")
			}
			return user, nil
		}
		if Debug {
			log.Println("Auth: invalid key cached")
		}
		return nil, ErrInvalidApiKey
	}

	payload := url.Values{}
	payload.Add("token", keyString)

	res, err := client.PostForm(fmt.Sprintf("%s/api/api-keys/check", authEndpoint), payload)
	if err != nil {
		// if we have an expired cached entry for the user, reset the cache expiration and return that
		// this allows the service to remain available if grafana.net is unreachable
		if user != nil {
			if Debug {
				log.Printf("Auth: re-caching validKey response for %d seconds", validTTL/time.Second)
			}
			cache.Set(keyString, user, validTTL)
			return user, nil
		}

		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if Debug {
		log.Printf("Auth: apiKey check response was: %s", body)
	}

	if res.StatusCode >= 500 {
		// if we have an expired cached entry for the user, reset the cache expiration and return that
		// this allows the service to remain available if grafana.net is unreachable
		if user != nil {
			if Debug {
				log.Printf("Auth: re-caching validKey response for %d seconds", validTTL/time.Second)
			}
			cache.Set(keyString, user, validTTL)
			return user, nil
		}
		return nil, fmt.Errorf("Auth request failed. status %d", res.StatusCode)
	}

	if res.StatusCode != 200 {
		// add the invalid key to the cache
		if Debug {
			log.Printf("Auth: Caching invalidKey response for %d seconds", invalidTTL/time.Second)
		}
		cache.Set(keyString, nil, invalidTTL)

		return nil, ErrInvalidApiKey
	}

	user = &SignedInUser{key: keyString}
	err = json.Unmarshal(body, user)
	if err != nil {
		return nil, err
	}

	valid := false
	// keeping it backwards compatible
	if len(validOrgIds) == 0 {
		valid = true
	} else {
		for _, id := range validOrgIds {
			if user.OrgId == id {
				valid = true
				break
			}
		}
	}

	if !valid {
		return nil, ErrInvalidOrgId
	}

	// add the user to the cache.
	if Debug {
		log.Printf("Auth: Caching validKey response for %d seconds", validTTL/time.Second)
	}
	cache.Set(keyString, user, validTTL)
	return user, nil
}
