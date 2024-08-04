package main

import (
	"fmt"
	"html/template"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"encoding/csv"
)

type TemplateRenderer struct {
	templates *template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	err := t.templates.ExecuteTemplate(w, name, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", name, err)
	}
	return err
}

var DB *gorm.DB

func Init() {
	var err error
	DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// データベースのマイグレーション
	DB.AutoMigrate(&User{})
	DB.AutoMigrate(&UserInfoMation{})
	DB.AutoMigrate(&AbsenceUser{})
}

type UserInfoMation struct {
	gorm.Model
	UserID        uint // ユーザーテーブルの主キー
	PhoneNumber   string
	QRCODE_Number string
	Attend        bool
}

var e = createMux()

var ScanUserData User

type User struct {
	gorm.Model
	//新郎側か新婦側か
	//新郎側がtrue
	MenOrWomen bool
	//氏名
	UserName string
	//電話番号
	PhoneNumber string
	// アレルギー情報
	AllergyInfo string
	// 同伴者
	Companion int

	ByBusFlag bool

	// Userが入力しない情報
	UserInfo UserInfoMation
}

type AbsenceUser struct{
	gorm.Model

	UserName string

	Message string
}

func main() {
	Init()

	e.Renderer = &TemplateRenderer{
		templates: template.Must(template.ParseGlob("../templates/*.html")),
	}

	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	e.GET("/absence",func(c echo.Context)error{
		return c.Render(http.StatusOK,"absence.html",nil)
	})

	// ログインフォームの表示用のルーティング
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", nil)
	})

	e.GET("/signup", func(c echo.Context) error {
		return c.Render(http.StatusOK, "signup.html", nil)
	})
	e.GET("/qrcode", func(c echo.Context) error {
		return c.Render(http.StatusOK, "QRCodeRead.html", nil)
	})
	
	e.GET("/dblogin",func(c echo.Context)error{
		return c.Render(http.StatusOK,"dblogin.html",nil)
	})

	e.GET("/export-csv", exportCSVHandler)

	e.Static("/css", "../css")
	e.Static("/js", "../js")
	e.Static("/images", "../images")
	e.Static("/QRCode", "../QRCode")

	// ログイン用のルーティング
	e.POST("/login", loginHandler)

	// ユーザー登録のルーティング
	e.POST("/signup", signupHandler)

	e.POST("/absenceinfo",absenceHandler)

	e.POST("/db",DbConnectionHandler)

	e.POST("/scanResult",QRScanHandler)

	e.POST("/confirmation",ConfirmationHandler)

	e.POST("/infoUpDate",InfoUpDateHandler)

	e.POST("/execute-query",ExecuteQueryHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("DEfaulting to port %s", port)
	}


	log.Printf("Listening on port %s", port)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", port)))
}

func CreateQRCode(phoneNumber string) string {
	qrCode, _ := qr.Encode(phoneNumber, qr.M, qr.Auto)
	qrCode, _ = barcode.Scale(qrCode, 200, 200)
	fileName := fmt.Sprintf("../QRCode/%s_qrcode.png", phoneNumber)
	file, _ := os.Create(fileName)
	defer file.Close()

	png.Encode(file, qrCode)
	return phoneNumber + "_qrcode.png"
}

func createMux() *echo.Echo {
	e := echo.New()
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())
	e.Use(middleware.Gzip())

	return e
}
func loginHandler(c echo.Context) error {
	phoneNumber := c.FormValue("phoneNumber")
	var user User
	if err := DB.Where("phone_number = ?", phoneNumber).Preload("UserInfo").First(&user).Error; err != nil {
		return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
			"Error": "Invalid credentials",
		})
	}
	if user.PhoneNumber == phoneNumber {
		return c.Render(http.StatusOK, "login_success.html", map[string]interface{}{
			"UserName": user.UserName,
			"Img":      user.UserInfo.QRCODE_Number,
			"MenOrWomenInfo":user.MenOrWomen,
			"PhoneNumber":user.PhoneNumber,
			"AllergyInfo":user.AllergyInfo,
			"Companion":user.Companion,
			"ByBusFlag":user.ByBusFlag,
		})
	}

	return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
		"Error": "Invalid username or phone number",
	})
}

func signupHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("phoneNumber")
	allergyInfo:=c.FormValue("allergyInfo")
	companion:=c.FormValue("companion")
	morw:=c.FormValue("morw")
	bybus := c.FormValue("bybus")

	var menOrwoman bool
	if morw != "" {
		menOrwoman = true
	}

	byBusFlag := false
	if bybus == "on" {
		byBusFlag = true
	}

	_, err := CreateUser(username, password, allergyInfo, companion, menOrwoman, byBusFlag)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create user")
	}
	return c.Render(http.StatusOK, "index.html", nil)
}

func CreateUser(username, phoneNumber, allergyInfo, companion string, menOrwoman, bybus bool) (*User, error) {
	numCompanion, _ := strconv.Atoi(companion)
	user := &User{UserName: username, PhoneNumber: phoneNumber, AllergyInfo: allergyInfo, Companion: numCompanion, MenOrWomen: menOrwoman, ByBusFlag: bybus}
	var userInfo UserInfoMation
	userInfo.PhoneNumber = user.PhoneNumber
	userInfo.QRCODE_Number = CreateQRCode(user.PhoneNumber)
	user.UserInfo = userInfo
	if err := DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func QRScanHandler(c echo.Context) error {
	// QRコードデータを取得
	qrCodeData := c.FormValue("qrCodeData")

	var user User

	// データベースからQRコードデータに一致するphoneNumberを持つUserを検索
	if err := DB.Where("phone_number = ?", qrCodeData).Preload("UserInfo").First(&user).Error; err != nil {
		// 一致するUserが見つからない場合の処理
		if err == gorm.ErrRecordNotFound {
			// ユーザーが見つからないエラーメッセージを返す
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		// その他のエラー
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}
	
	ScanUserData=user

	// 成功した場合、クライアントに対して成功のメッセージを返す
	return c.Render(http.StatusOK,"confirmation.html",map[string]interface{}{
		"UserName":user.UserName,
	})
}


func DbConnectionHandler(c echo.Context)error{
	passWord:=c.FormValue("password")
	if(passWord=="kento1201"){
		return DbOpenHandler(c)
	}
	return nil
}


	func DbOpenHandler(c echo.Context) error {
		var users []User
		var userInfos []UserInfoMation
		var absence []AbsenceUser
		// ユーザーデータを取得
		if err := DB.Find(&users).Error; err != nil {
			log.Printf("Failed to get users: %v", err)
		}
	
		// ユーザー情報を取得
		if err := DB.Find(&userInfos).Error; err != nil {
			log.Printf("Failed to get user infos: %v", err)
		}

		if err:=DB.Find(&absence).Error;err!=nil{
			log.Printf("Faild to get absenceUser:%v",err)
		}

		return c.Render(http.StatusOK, "dbhome.html", map[string]interface{}{
			"Users":      users,
			"UserInfos":  userInfos,
			"absence":absence,
		})
	}


func ConfirmationHandler(c echo.Context)error{
    // UserInfoのAttendフィールドをtrueに更新
    ScanUserData.UserInfo.Attend = true
    if err := DB.Save(&ScanUserData.UserInfo).Error; err != nil {
        // データベース更新時のエラー処理
        return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user info"})
    }
	userName:=ScanUserData.UserName
	var user User
	ScanUserData=user
    return c.Render(http.StatusOK,"confirmationTrue.html",map[string]interface{}{
		"UserName":userName,
	})
}

func InfoUpDateHandler(c echo.Context) error {
    morw := c.FormValue("morw") // チェックボックスの値を取得
    userName := c.FormValue("username")
    phoneNumber := c.FormValue("phoneNumber")
    allergyInfo := c.FormValue("allergyInfo")
    companion := c.FormValue("companion")
	bybus:=c.FormValue("bybus")

    var menOrWomen bool
    if morw == "on" {
        menOrWomen = true
    } else {
        menOrWomen = false
    }

	byBusFlag := false
	if bybus == "on" {
		byBusFlag = true
	}

    num, err := strconv.Atoi(companion)
    if err != nil {
        return c.JSON(http.StatusBadRequest, map[string]interface{}{
            "ErrorCode_UpDateHandler3": "同伴者数が無効です。",
        })
    }

    // 更新対象のユーザーをデータベースから取得
    var user User
    if err := DB.Where("phone_number = ?", phoneNumber).First(&user).Error; err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "ErrorCode_UpDateHandler1": "ユーザーが見つかりません。",
        })
    }

    // 更新内容を設定
	if userName!=""{
		user.UserName = userName
	}
	if allergyInfo!=""{
		user.AllergyInfo = allergyInfo

	}
    user.Companion = num
    user.MenOrWomen = menOrWomen
	user.ByBusFlag=byBusFlag

    // データベースに保存
    if err := DB.Save(&user).Error; err != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "ErrorCode_UpDateHandler4": "更新に失敗しました。",
        })
    }

    return c.JSON(http.StatusOK, map[string]interface{}{
        "message": user.UserName + "さんの情報が更新されました。",
    })
}


func ExecuteQueryHandler(c echo.Context) error {
    query := c.FormValue("sqlQuery")

    // クエリを実行
    result := DB.Exec(query)
    if result.Error != nil {
        return c.JSON(http.StatusInternalServerError, map[string]interface{}{
            "error":   "Failed to execute query",
            "details": result.Error.Error(),
        })
    }

    // 結果を取得
    rowsAffected := result.RowsAffected

    return c.JSON(http.StatusOK, map[string]interface{}{
        "message":       "Query executed successfully",
        "rows_affected": rowsAffected,
    })
}


func exportCSVHandler(c echo.Context) error {
    // ユーザーとその関連情報を取得
    var users []User
    DB.Preload("UserInfo").Find(&users) // Preload を使用して関連する UserInfo を事前にロード

    // CSVファイルをメモリ上に作成
    records := [][]string{
        {"MenOrWomen", "UserName", "PhoneNumber", "AllergyInfo", "Companion","ByBusFlag", "UserID", "UserPhoneNumber", "QRCODE_Number", "Attend"},
    }

    for _, user := range users {
        menOrWomen := "新婦側"
        if user.MenOrWomen {
            menOrWomen = "新郎側"
        }
        // UserInfo データ
        userID := fmt.Sprintf("%d", user.UserInfo.UserID)
        userPhoneNumber := user.UserInfo.PhoneNumber
        qrcodeNumber := user.UserInfo.QRCODE_Number
        attend := fmt.Sprintf("%t", user.UserInfo.Attend)
        byBusFlag := fmt.Sprintf("%t", user.ByBusFlag) // ByBusFlagを文字列に変換

        // レコードの追加
        records = append(records, []string{
            menOrWomen,
            user.UserName,
            user.PhoneNumber,
            user.AllergyInfo,
            fmt.Sprintf("%d", user.Companion),
			byBusFlag,
			userID,
            userPhoneNumber,
            qrcodeNumber,
            attend,
        })
    }

    // レスポンスヘッダの設定
    c.Response().Header().Set("Content-Type", "text/csv")
    c.Response().Header().Set("Content-Disposition", "attachment; filename=\"users.csv\"")
    w := csv.NewWriter(c.Response().Writer)
    w.WriteAll(records) // CSVデータの書き込み
    w.Flush()

    return nil
}


func absenceHandler(c echo.Context)error{
	username := c.FormValue("username")
	message:=c.FormValue("message")
	
	_,err := CreateAbsenceUser(username,message)
	if err!=nil{
		return c.String(http.StatusInternalServerError, "Failed to create user")
	}
	return c.Render(http.StatusOK, "index.html",nil)
}

func CreateAbsenceUser(username,message string)(*AbsenceUser,error){
	user := &AbsenceUser{UserName: username,Message:message}
	if err := DB.Create(user).Error; err != nil {
		return nil,err
	}
	return user,nil
}