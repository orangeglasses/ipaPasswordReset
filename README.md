# ipaPasswordReset
This a self-service password reset tool for Free IPA and RedHat IDM

# How it works
This app will present a webform where a user can enter his/her username to request a password reset. A token will be generated and stored in the redis back-end.
The user will receive an e-mail on the address which is registered in IPA for this account. The e-mail will contain a link which the user has to open within the TokenValidity time (default: 5 minutes).
When the user opens the link he'll get the opportunity to enter a new password.

#  How to deploy
This app is developed to run on cloudfoundry but it should run anywhere. The following environment variables need to be set:
- PWRESET_IPAHOST (an IPA server)
- PWRESET_IPAUSER (IPA user with sufficient permissions to reset passwords)
- PWRESET_IPAPASSWORD (Password for that user)
- PWRESET_EMAILHOST (SMTP Host)
- PWRESET_EMAILPORT (SMTP Port)
- PWRESET_EMAILFROM (From Address)
- PWRESET_EMAILUSER (optional, SMTP User for authenticated SMTP)
- PWRESET_EMAILPASSWORD (optional, Password for that user)
- PWRESET_APPNAME (optional, Name of the app. This will show up in the browser as the name of the page. Default: IPA Password Reset SelfService)
- PWRESET_TOKENVALIDITY (optional, Validity of the token in minutes. Default: 5 minutes)
- PWRESET_BLOCKEDGROUPS (optional, Member of these groups will not be allowed to user this service to reset their password)
- PWRESET_BLOCKEDPREFIXES (optional, user accounts starting with these characters will not be allow to use this service to reset their password. Multiple prefixes possible, comma seperated)
- PWRESET_SERVICEACCOUNTPREFIXES (optional, see below)
- PWRESET_MINPASSWORDLENGTH (optional, Minimum password length. default: 12)
- PWRESET_SVCACCPASSWORDLENGTH (optional, length for serviceaccount passwords)
- PWRESET_USERPWVALIDITYMONTHS (optional, expiration time of the new password in months. default: 3 months)
- PWRESET_SVCACCPWVALIDITYMONTHS (optional, expiration time of service account passwords in months. default: 12 months)

If you run on cloudfoundry you need to bind a redis service to this app. If you're not running in CF you can point this app to a redis instance using the following env vars:
- PWRESET_REDISHOST
- PWRESET_REDISPORT
- PWRESET_REDISPASSWORD
- PWRESET_REDISDB

If this app runs outside CF then you might want to configure the post this app is running on using PWRESET_APPPORT. The default is port 9000

#Service account password generation
This tool offers two way to reset a password. The first way is what is usually used for regular user accounts and allows a user to enter a new password.
The second way this tool can reset passwords is by generating a password and displaying that once. This works similar to how API keys are usually retrieved. 
This method is what most security policies require for so called "service accounts". To use this feature you need to set the PWRESET_SERVICEACCOUNTPREFIXES env var.
This tells the tool to use this method for account which start with the characters configured in that env var. You can configure multiple prefixes in a comma seperated list.
