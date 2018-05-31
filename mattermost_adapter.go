package ldapsync

import (
	"github.com/mattermost/mattermost-server/model"

	"log"
	"strconv"
	"strings"
)

func (this *LDAPAuthenticatorWithSync) checkMattermostUser(id int64, username, name, mail string) {
	user, resp := this.mattermost.GetUserByEmail(mail, "")
	if resp.Error != nil && resp.StatusCode != 404 {
		log.Fatalf("ERROR: %+v", resp.Error)
	}

	created := false
	userId := strconv.FormatInt(id, 10)
	if resp.StatusCode == 404 {
		log.Println("Creating new user.")
		// This user does not exist
		var newUser model.User
		newUser.AuthService = model.USER_AUTH_SERVICE_GITLAB
		newUser.AuthData = &userId
		newUser.Email = mail
		newUser.FirstName = name
		newUser.Username = username
		newUser.EmailVerified = true

		user, resp = this.mattermost.CreateUser(&newUser)
		if resp.Error != nil {
			log.Printf("Could not create user with email %s, got error: %+v.", mail, resp.Error)
		}

		created = true
	}

	// Update user if not just created
	if !created {
		user.Username = username
		user.Email = mail
		user.FirstName = strings.Split(name, " ")[0]
		if len(strings.Split(name, " ")) > 1 {
			user.LastName = strings.Split(name, " ")[1]
		}

		_, resp = this.mattermost.UpdateUser(user)
		if resp.Error != nil {
			log.Printf("Could not update existing user, got Error %+v", resp.Error)
		}
	}

}

func (this *LDAPAuthenticatorWithSync) checkGroupForMattermostUser(group Group, mail string) {
	group.uid = strings.Replace(group.uid, "_", "-", -1)
	team, resp := this.mattermost.GetTeamByName(group.uid, "")
	if resp.Error != nil && resp.StatusCode != 404 {
		log.Printf("ERROR: Could not find team %+v, got error: %+v.", group, resp.Error)
	}

	if resp.StatusCode == 404 {
		newTeam := model.Team{}
		newTeam.Name = strings.Replace(group.uid, "_", "-", -1)
		newTeam.DisplayName = group.name
		newTeam.Type = "I"
		team, resp = this.mattermost.CreateTeam(&newTeam)
		if resp.Error != nil {
			log.Printf("ERROR: Could not create Team %+v, got error %+v", group, resp.Error)
			return
		}

		log.Printf("Created new Team %s.\n", team.DisplayName)
	}

	user, userResp := this.mattermost.GetUserByEmail(mail, "")
	if userResp.Error != nil {
		log.Printf("ERROR: Could not fetch user when adding to team %+v, got error: %+v", group, userResp.Error)
		return
	}

	_, err := this.mattermost.AddTeamMember(team.Id, user.Id)
	if err.Error != nil {
		log.Printf("ERROR: Could add user to team %+v, got error: %+v", group, err.Error)
		return
	}

	log.Printf("Added user %s to team %s \n", user.Email, team.DisplayName)
}
