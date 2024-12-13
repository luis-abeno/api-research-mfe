package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Question struct {
	ID       int    `json:"id"`
	GroupID  int    `json:"group_id"`
	Question string `json:"question"`
}

type Group struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	err := godotenv.Load()
    if err != nil {
        log.Println("No .env file found")
    }

    connStr := os.Getenv("DATABASE_URL")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	r := gin.Default()

	// Add CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"https://research-mfe.vercel.app/"},
		AllowMethods: []string{"GET", "POST"},
		AllowHeaders: []string{"Origin", "Content-Type"},
	}))

	r.GET("/", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "Health check passed"})
    })

	r.GET("/questions", func(c *gin.Context) {
		rows, err := db.Query(`
			SELECT q.id, q.id_group, q.question, g.id, g.name 
			FROM questions q 
			JOIN groups g ON q.id_group = g.id
			ORDER BY q.id_group, q.id
		`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		questions := make(map[int][]Question)
		groups := make(map[int]Group)
		for rows.Next() {
			var q Question
			var g Group
			if err := rows.Scan(&q.ID, &q.GroupID, &q.Question, &g.ID, &g.Name); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			questions[q.GroupID] = append(questions[q.GroupID], q)
			groups[g.ID] = g
		}

		groupList := []gin.H{}
		for groupID, group := range groups {
			groupList = append(groupList, gin.H{
				"id":        groupID,
				"name":      group.Name,
				"questions": questions[groupID],
			})
		}

		c.JSON(http.StatusOK, gin.H{"groups": groupList})
	})

	r.POST("/save-answers", func(c *gin.Context) {
		var payload struct {
			Answers []struct {
				IDQuestion int `json:"id_question"`
				Answer     struct {
					ID    int    `json:"id"`
					Value string `json:"value"`
				} `json:"answer"`
				FullName string `json:"full_name"`
				Email	string `json:"email"`
				Role     string `json:"role"`
			} `json:"answers"`
		}

		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		tx, err := db.Begin()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		stmt, err := tx.Prepare(`
			INSERT INTO answers (id_question, answer, full_name, email, role)
			VALUES ($1, $2, $3, $4, $5)
		`)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer stmt.Close()

		for _, answer := range payload.Answers {
			if _, err := stmt.Exec(answer.IDQuestion, answer.Answer.ID, answer.FullName, answer.Email, answer.Role); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "answers saved"})
	})

	r.Run(":8080")
}