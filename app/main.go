package main

import(
	"net/http"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"html/template"
	"io"
	"log"
)

type TemplateRenderer struct{
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", name, err)
	}
	return err
}


var e=createMux()




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
func main(){
	e.Renderer=&TemplateRenderer{
		templates:template.Must(template.ParseGlob("../templates/*.html")),
	}

	e.GET("/login",func(c echo.Context)error{
		return c.Render(http.StatusOK,"login.html",nil)
	})


	e.GET("/main",func(c echo.Context)error{
		return c.Render(http.StatusOK,"index.html",nil)
	})

	e.POST("/main",loginHandler)

	e.Logger.Fatal(e.Start(":8080"))
}



func createMux()*echo.Echo{
	e:=echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.Gzip())

	return e
}

func loginHandler(c echo.Context)error{
	username:=c.FormValue("username")
	phoneNumber:=c.FormValue("phoneNumber")
	// user:=User.CreateUser(username,phoneNumber)
	user:=CreateUser(username,phoneNumber)
	if user.UserName=="test"&&user.PhoneNumber=="111"{
		return c.Render(http.StatusOK,"index.html",nil)
	}else{
		return c.String(http.StatusUnauthorized, "Invalid username or phone number")
	}
}