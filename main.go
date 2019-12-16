/*
Copyright (C) 2019 Synopsys, Inc.

Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements. See the NOTICE file
distributed with this work for additional information
regarding copyright ownership. The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	// import postgresql
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	namespace          = os.Getenv("POD_NAMESPACE")
	httpReadinessURLs  = ""
	postgresDBHost     = ""
	postgresDBPort     = 5432
	postgresDB         = "postgres"
	postgresDBUser     = ""
	postgresDBPassword = ""
	postgresDBSSLMode  = "disable"
	mongoDBHost        = ""
	mongoDBPort        = 27017
	mongoDB            = ""
	mongoDBUser        = ""
	mongoDBPassword    = ""
)

const (
	postgresDefaultQuery = "SELECT datname FROM pg_database WHERE datistemplate = false"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "init",
	Short: "init is a command line tool used for Polaris readiness check for pods",
	Args: func(cmd *cobra.Command, args []string) error {
		// Check number of arguments
		if len(args) > 0 {
			cmd.Help()
			return fmt.Errorf("this command doesn't take any arguments")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// validate HTTP readiness check. If HTTP readiness check fails, wait for 5 secs and try again
		for {
			err := validateHTTPReadinessCheckCommands()
			if err == nil {
				break
			}
			log.Errorf("unable to validate readiness check commands due to %+v, trying again after 5 secs", err)
			time.Sleep(5 * time.Second)
		}

		// validate postgres database connection. If postgres connection fails, wait for 5 secs and try again
		for {
			err := validatePostgresDBConnection()
			if err == nil {
				break
			}
			log.Errorf("unable to validate postgres database connection due to %+v, trying again after 5 secs", err)
			time.Sleep(5 * time.Second)
		}

		// validate mongo database connection. If mongo connection fails, wait for 5 secs and try again
		for {
			err := validateMongoDBConnection()
			if err == nil {
				break
			}
			log.Errorf("unable to validate mongo database connection due to %+v, trying again after 5 secs", err)
			time.Sleep(5 * time.Second)
		}

		return nil
	},
}

func validateHTTPReadinessCheckCommands() error {
	if len(httpReadinessURLs) != 0 {
		log.Info("validating HTTP readiness check")
		httpReadinessURLArr := strings.Split(httpReadinessURLs, ",")

		for _, httpReadinessURL := range httpReadinessURLArr {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}

			httpClient := &http.Client{
				Timeout:   10 * time.Second,
				Transport: tr,
			}

			resp, err := httpClient.Get(strings.TrimSpace(httpReadinessURL))
			if err != nil {
				return fmt.Errorf("get failed for %+v", err)
			}
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Infof("response: %s", string(body))
				return fmt.Errorf("reading response failed for %+v", err)
			}
			log.Infof("output: %s", string(body))
		}
		log.Info("all HTTP readiness check passed....")
	} else {
		log.Info("skipping HTTP readiness check")
	}
	return nil
}

func validatePostgresDBConnection() error {
	if len(postgresDBUser) != 0 && len(postgresDBPassword) != 0 {
		log.Info("validating postgres database connection")
		// form psql connection info
		if len(postgresDBSSLMode) == 0 {
			postgresDBSSLMode = "disable"
		}

		switch strings.ToLower(postgresDBSSLMode) {
		case "true":
			postgresDBSSLMode = "require"
		case "false":
			postgresDBSSLMode = "disable"
		}
		psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
			postgresDBHost, postgresDBPort, postgresDBUser, postgresDBPassword, postgresDB, postgresDBSSLMode)
		// open postgres connection
		db, err := sql.Open("postgres", psqlInfo)
		if err != nil {
			return fmt.Errorf("connection failed for %+v", err)
		}
		defer db.Close()

		// wait for postgres database to be accessible
		for i := 0; i < 10; i++ {
			// ping the database connection
			err = db.Ping()
			if err == nil {
				break
			}

			// if the count reaches the maximum count for ping, error out
			if i == 10 {
				return fmt.Errorf("ping failed for %+v", err)
			}
			time.Sleep(5 * time.Second)
		}

		// validate whether it able to execute default query
		result, err := db.Exec(postgresDefaultQuery)
		if err != nil {
			return fmt.Errorf("exec statement failed for %+v", err)
		}

		nbRow, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if nbRow == 0 {
			return fmt.Errorf("rows affected 0 for the query: %s", postgresDefaultQuery)
		}
		log.Infof("postgres database '%s' instance is up and running....", postgresDBHost)
	} else {
		log.Info("skipping postgres database readiness check")
	}
	return nil
}

func validateMongoDBConnection() error {
	if len(mongoDB) != 0 && len(mongoDBUser) != 0 && len(mongoDBPassword) != 0 {
		log.Info("validating mongo database connection")
		// Set client options
		clientOptions := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@%s:%d/%s", mongoDBUser, mongoDBPassword, mongoDBHost, mongoDBPort, mongoDB))
		// Connect to MongoDB
		client, err := mongo.Connect(context.TODO(), clientOptions)
		if err != nil {
			return fmt.Errorf("connection failed for %+v", err)
		}

		// wait for mongo database to be accessible
		for i := 0; i < 10; i++ {
			err = client.Ping(context.TODO(), nil)
			if err == nil {
				break
			}
			// if the count reaches the maximum count for ping, error out
			if i == 10 {
				return fmt.Errorf("ping failed for %+v", err)
			}
			time.Sleep(5 * time.Second)
		}
		log.Infof("mongo database '%s' instance is up and running....", mongoDBHost)
	} else {
		log.Info("skipping mongo database readiness check")
	}
	return nil
}

func init() {
	if len(namespace) == 0 {
		namespace = "default"
	}
	postgresDBHost = fmt.Sprintf("postgresql.%s.svc.cluster.local", namespace)
	mongoDBHost = fmt.Sprintf("mongodb.%s.svc.cluster.local", namespace)
	rootCmd.Flags().StringVarP(&httpReadinessURLs, "http-readiness-check-urls", "c", httpReadinessURLs, "HTTP readiness check URL's separated by ','")
	rootCmd.Flags().StringVarP(&postgresDBHost, "postgres-host", "s", postgresDBHost, "Postgres database host")
	rootCmd.Flags().IntVarP(&postgresDBPort, "postgres-port", "o", postgresDBPort, "Postgres database port")
	rootCmd.Flags().StringVarP(&postgresDB, "postgres-database", "b", postgresDB, "Postgres database name")
	rootCmd.Flags().StringVarP(&postgresDBUser, "postgres-user", "u", postgresDBUser, "Postgres database user")
	rootCmd.Flags().StringVarP(&postgresDBPassword, "postgres-password", "p", postgresDBPassword, "Postgres database password")
	rootCmd.Flags().StringVarP(&postgresDBSSLMode, "postgres-ssl-mode", "l", postgresDBSSLMode, "Postgres database SSL mode")
	rootCmd.Flags().StringVarP(&mongoDBHost, "mongo-host", "m", mongoDBHost, "Mongo database host")
	rootCmd.Flags().IntVarP(&mongoDBPort, "mongo-port", "r", mongoDBPort, "Mongo database port")
	rootCmd.Flags().StringVarP(&mongoDB, "mongo-database", "g", mongoDB, "Mongo database name")
	rootCmd.Flags().StringVarP(&mongoDBUser, "mongo-user", "e", mongoDBUser, "Mongo database user")
	rootCmd.Flags().StringVarP(&mongoDBPassword, "mongo-password", "d", mongoDBPassword, "Mongo database password")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("polaris init failed: %+v", err)
		os.Exit(1)
	}
}
