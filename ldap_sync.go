package ldapsync

import (
	"errors"
	"fmt"
	"log"

	"github.com/go-ldap/ldap"
	"github.com/mattermost/mattermost-server/model"
	lauth "github.com/zonradkuse/go-ldap-authenticator"
)

type Group struct {
	uid  string
	name string
}

type LDAPAuthenticatorWithSync struct {
	authenticator *lauth.LDAPAuthenticator

	userDn string

	mattermost *model.Client4
}

func NewLDAPAuthenticator(bindDn, bindPassword, queryDn string, selectors []string, transformer lauth.LDAPTransformer) LDAPAuthenticatorWithSync {
	var syncAuther LDAPAuthenticatorWithSync
	auther := lauth.NewLDAPAuthenticator(bindDn, bindPassword, queryDn, selectors, transformer)
	syncAuther.authenticator = &auther
	syncAuther.userDn = queryDn

	return syncAuther
}

func (this *LDAPAuthenticatorWithSync) Connect(bindUrl string) error {
	return this.authenticator.Connect(bindUrl)
}

func (this *LDAPAuthenticatorWithSync) Close() {
	this.authenticator.Close()
}

func (this *LDAPAuthenticatorWithSync) ConnectMattermost(url, username, password string) error {
	this.mattermost = model.NewAPIv4Client(url)
	_, resp := this.mattermost.Login(username, password)

	if resp.Error != nil {
		log.Printf("Got error during login: %+v\n", resp.Error)
		return errors.New("Login failed.")
	}

	return nil
}

func (this LDAPAuthenticatorWithSync) GetUserById(id string) (error, interface{}) {
	return this.authenticator.GetUserById(id)
}

func (this LDAPAuthenticatorWithSync) Authenticate(username, password string) (error, string) {
	err, uid := this.authenticator.Authenticate(username, password)
	if err != nil {
		return err, ""
	}

	this.syncMattermostForUser(uid)
	return nil, uid
}

func (this *LDAPAuthenticatorWithSync) fetchGroupsForUser(uid string) []Group {
	conn := this.authenticator.GetConnection()

	filter := fmt.Sprintf("(&(objectClass=*)(member=uid=%s,%s))", uid, this.userDn)
	searchRequest := ldap.NewSearchRequest(
		"dc=sog", // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		filter, // The filter to apply
		[]string{"dn", "cn", "ou"}, // A list attributes to retrieve
		nil,
	)

	res, err := conn.Search(searchRequest)
	if err != nil {
		log.Println("ERROR: %+v", err)
		return []Group{}
	}

	entries := res.Entries
	var groups []Group
	for _, entry := range entries {
		group := Group{uid: entry.GetAttributeValue("ou"), name: entry.GetAttributeValue("cn")}
		groups = append(groups, group)
	}

	return groups
}

func (this *LDAPAuthenticatorWithSync) syncMattermostForUser(uid string) {
	err, user := this.GetUserById(uid)
	if err != nil {
		log.Println("ERROR: %+v", err)
		return
	}

	this.checkMattermostUser(user.(UserData).Id, user.(UserData).Username, user.(UserData).Name, user.(UserData).Email)

	groups := this.fetchGroupsForUser(uid)
	for _, group := range groups {
		this.checkGroupForMattermostUser(group, user.(UserData).Email)
	}
}
