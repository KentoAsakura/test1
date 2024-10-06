package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
    log.Println("Opening database connection...") // 追加
    DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
    if err != nil {
        log.Fatalf("Failed to connect database: %v", err) // エラーログ
    }
    log.Println("Database connection successful.") // 追加
    
    // データベースのマイグレーション
    log.Println("Migrating database schemas...") // 追加
    DB.AutoMigrate(&User{})
    DB.AutoMigrate(&UserInfoMation{})
    DB.AutoMigrate(&AbsenceUser{})
    log.Println("Database migration complete.") // 追加

    // サーバー起動時にmatchedPhoneNumbersをファイルから読み込む
    log.Println("Loading matched phone numbers...") // 追加
    if err := loadMatchedPhoneNumbers(); err != nil {
        log.Fatalf("Error loading matched phone numbers: %v", err) // エラーログ
    }
    log.Println("Matched phone numbers loaded successfully.") // 追加
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

type AbsenceUser struct {
	gorm.Model

	UserName string

	Message string
}

type CsvUser struct {
	gorm.Model

	Table       string
	Name        string
	Money       string
	PhoneNumber string
	Sound       bool
}

var matchedPhoneNumbers []string

var mutex sync.Mutex

const matchedPhoneNumbersFile = "app/matchedPhoneNumbers.json"

func main() {
// 初期化時にログ出力
log.Println("Initializing the database and server setup...")

Init()
    // カスタム関数をテンプレートに登録
    funcMap := template.FuncMap{
        "contains": contains,
    }
log.Println("Server initialization complete, setting up routes...")


e.Renderer = &TemplateRenderer{
	templates: template.Must(template.New("templates").Funcs(funcMap).ParseGlob("templates/*.html")),
}


	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})

	e.GET("/absence", func(c echo.Context) error {
		return c.Render(http.StatusOK, "absence.html", nil)
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

	e.GET("/dblogin", func(c echo.Context) error {
		return c.Render(http.StatusOK, "dblogin.html", nil)
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

	e.POST("/absenceinfo", absenceHandler)

	e.POST("/db", DbConnectionHandler)

	e.POST("/scanResult", QRScanHandler)

	e.POST("/confirmation", ConfirmationHandler)

	e.POST("/infoUpDate", InfoUpDateHandler)

	e.POST("/execute-query", ExecuteQueryHandler)
	// 電話番号をPOSTで受け取りリストにリダイレクトするエンドポイント
	e.POST("/api/attendance/:phoneNumber", postPhoneNumberHandler)

	// ユーザーリストを表示するエンドポイント
	e.GET("/api/attendance/list", listUsersHandler)
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

// CSVファイルを読み込んでUserのリストを返す関数
func readCSVFile(filePath string) ([]CsvUser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var users []CsvUser
	for i, record := range records {
		// 1行目はヘッダーなのでスキップ
		if i == 0 {
			continue
		}

		sound := false
		if record[4] == "true" {
			sound = true
		}

		user := CsvUser{
			Table:       record[0],
			Name:        record[1],
			Money:       record[2],
			PhoneNumber: record[3],
			Sound:       sound,
		}
		users = append(users, user)
	}

	return users, nil
}

func saveMatchedPhoneNumbers() error {
    log.Println("Saving matched phone numbers to file...")
    mutex.Lock()
    defer mutex.Unlock()

    log.Println("Marshalling matched phone numbers...")
    data, err := json.Marshal(matchedPhoneNumbers)
    if err != nil {
        log.Printf("Error marshalling matched phone numbers: %v", err)
        return err
    }

    // ファイルパスを絶対パスに変更
    absolutePath, err := filepath.Abs(matchedPhoneNumbersFile)
    if err != nil {
        log.Printf("Error getting absolute file path: %v", err)
        return err
    }

    log.Printf("Attempting to write to file: %s", absolutePath)
    err = ioutil.WriteFile(absolutePath, data, 0644)
    if err != nil {
        log.Printf("Error writing matched phone numbers to file: %v", err) // ここでエラーメッセージを確認
        return err
    }

    log.Println("Successfully saved matched phone numbers to file.")
    return nil
}



// 外部ファイルからmatchedPhoneNumbersを読み込む関数
func loadMatchedPhoneNumbers() error {
    log.Println("Acquiring lock for matched phone numbers...") // ログ追加
    mutex.Lock()
    defer mutex.Unlock()

    log.Println("Reading matched phone numbers file...") // ログ追加
    data, err := ioutil.ReadFile(matchedPhoneNumbersFile)
    if err != nil {
        if os.IsNotExist(err) {
            log.Println("Matched phone numbers file does not exist, creating a new one...") // ログ追加
            matchedPhoneNumbers = []string{}
            if err := saveMatchedPhoneNumbers(); err != nil {
                return err
            }
            return nil
        }
        return err
    }

    if len(data) == 0 {
        log.Println("Matched phone numbers file is empty.") // ログ追加
        matchedPhoneNumbers = []string{}
        return nil
    }

    log.Println("Unmarshalling matched phone numbers...") // ログ追加
    err = json.Unmarshal(data, &matchedPhoneNumbers)
    if err != nil {
        return err
    }

    log.Println("Successfully loaded matched phone numbers.") // ログ追加
    return nil
}


// 電話番号をPOSTで受け取り、保存した後、/api/attendance/listへリダイレクトする
func postPhoneNumberHandler(c echo.Context) error {
	// URLから電話番号を取得
	phoneNumber := c.Param("phoneNumber")

	// 受け取った電話番号を保存 (既に保存されていないか確認)
	mutex.Lock()
	if !contains(matchedPhoneNumbers, phoneNumber) {
		matchedPhoneNumbers = append(matchedPhoneNumbers, phoneNumber)
	}
	mutex.Unlock()

	// 外部ファイルに保存
	if err := saveMatchedPhoneNumbers(); err != nil {
		return c.String(http.StatusInternalServerError, "Error saving matched phone numbers")
	}

	// /api/attendance/list へリダイレクトして一覧表示を更新
	return c.Redirect(http.StatusSeeOther, "/api/attendance/list")
}

// ユーザーリストを一覧表示し、一致する電話番号の行に色を付けるハンドラ
func listUsersHandler(c echo.Context) error {
	users, err := readCSVFile("app/userList.csv") // CSVファイルのパスを指定
	if err != nil {
		return c.String(http.StatusInternalServerError, "Error reading CSV file")
	}

	// テンプレートに contains 関数を追加
	tmpl := template.Must(template.New("template.html").Funcs(template.FuncMap{
		"contains": contains,
	}).ParseFiles("templates/template.html"))

	// テンプレートにデータを渡してHTMLを生成
	data := struct {
		Users               []CsvUser
		MatchedPhoneNumbers []string
	}{
		Users:               users,
		MatchedPhoneNumbers: matchedPhoneNumbers,
	}

	return tmpl.Execute(c.Response().Writer, data)
}

// スライス内に電話番号が存在するか確認する関数
func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
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
			"UserName":       user.UserName,
			"Img":            user.UserInfo.QRCODE_Number,
			"MenOrWomenInfo": user.MenOrWomen,
			"PhoneNumber":    user.PhoneNumber,
			"AllergyInfo":    user.AllergyInfo,
			"Companion":      user.Companion,
			"ByBusFlag":      user.ByBusFlag,
		})
	}

	return c.Render(http.StatusUnauthorized, "login.html", map[string]interface{}{
		"Error": "Invalid username or phone number",
	})
}

func signupHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("phoneNumber")
	allergyInfo := c.FormValue("allergyInfo")
	companion := c.FormValue("companion")
	morw := c.FormValue("morw")
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

	ScanUserData = user

	// 成功した場合、クライアントに対して成功のメッセージを返す
	return c.Render(http.StatusOK, "confirmation.html", map[string]interface{}{
		"UserName": user.UserName,
	})
}

func DbConnectionHandler(c echo.Context) error {
	passWord := c.FormValue("password")
	if passWord == "kento1201" {
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

	if err := DB.Find(&absence).Error; err != nil {
		log.Printf("Faild to get absenceUser:%v", err)
	}

	return c.Render(http.StatusOK, "dbhome.html", map[string]interface{}{
		"Users":     users,
		"UserInfos": userInfos,
		"absence":   absence,
	})
}

func ConfirmationHandler(c echo.Context) error {
	// UserInfoのAttendフィールドをtrueに更新
	ScanUserData.UserInfo.Attend = true
	if err := DB.Save(&ScanUserData.UserInfo).Error; err != nil {
		// データベース更新時のエラー処理
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user info"})
	}
	userName := ScanUserData.UserName
	var user User
	ScanUserData = user
	return c.Render(http.StatusOK, "confirmationTrue.html", map[string]interface{}{
		"UserName": userName,
	})
}

func InfoUpDateHandler(c echo.Context) error {
	morw := c.FormValue("morw") // チェックボックスの値を取得
	userName := c.FormValue("username")
	phoneNumber := c.FormValue("phoneNumber")
	allergyInfo := c.FormValue("allergyInfo")
	companion := c.FormValue("companion")
	bybus := c.FormValue("bybus")

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
	if userName != "" {
		user.UserName = userName
	}
	if allergyInfo != "" {
		user.AllergyInfo = allergyInfo

	}
	user.Companion = num
	user.MenOrWomen = menOrWomen
	user.ByBusFlag = byBusFlag

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
		{"MenOrWomen", "UserName", "PhoneNumber", "AllergyInfo", "Companion", "ByBusFlag", "UserID", "UserPhoneNumber", "QRCODE_Number", "Attend"},
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

func absenceHandler(c echo.Context) error {
	username := c.FormValue("username")
	message := c.FormValue("message")

	_, err := CreateAbsenceUser(username, message)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create user")
	}
	return c.Render(http.StatusOK, "index.html", nil)
}

func CreateAbsenceUser(username, message string) (*AbsenceUser, error) {
	user := &AbsenceUser{UserName: username, Message: message}
	if err := DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}
