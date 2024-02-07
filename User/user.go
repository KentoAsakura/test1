package User

import(

)



type User struct{
	UserName string
	PhoneNumber string
}



func CreateUser(name string,phoneNumber string)User{
	var user User
	user.UserName=name
	user.PhoneNumber=phoneNumber
	return user
}