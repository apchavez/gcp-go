// Jenkins Declarative Pipeline equivalent of .github/workflows/ci.yml.
// Kept alongside GitHub Actions (the CI actually enforced on this repo) to demonstrate
// Jenkinsfile/Groovy DSL fluency for orgs that run Jenkins on-prem instead of a SaaS CI.
pipeline {
    agent any

    tools {
        go 'go1.23'
    }

    environment {
        SONAR_TOKEN = credentials('sonar-token')
        GO111MODULE = 'on'
    }

    options {
        timestamps()
        disableConcurrentBuilds()
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Trivy vulnerability scan') {
            steps {
                sh '''
                    trivy fs --scanners vuln --severity CRITICAL,HIGH \
                        --exit-code 1 --ignore-unfixed --format table .
                '''
            }
        }

        stage('Lint (golangci-lint)') {
            steps {
                sh 'golangci-lint run ./...'
            }
        }

        stage('Run tests') {
            steps {
                sh 'go test ./... -race'
            }
        }

        stage('Start Firestore emulator') {
            steps {
                sh '''
                    sudo apt-get update
                    sudo apt-get install -y google-cloud-cli-firestore-emulator
                    gcloud emulators firestore start --host-port=127.0.0.1:8080 &>/tmp/firestore-emulator.log &
                    for i in $(seq 1 30); do
                        curl -sf http://127.0.0.1:8080 >/dev/null 2>&1 && exit 0
                        sleep 1
                    done
                    exit 1
                '''
            }
        }

        stage('Compute coverage report') {
            environment {
                FIRESTORE_EMULATOR_HOST = '127.0.0.1:8080'
            }
            steps {
                sh '''
                    pkgs=$(go list ./... | grep -v '/db/migration$' | grep -v '/internal/testutil$' | paste -sd,)
                    go test $(echo $pkgs | tr ',' ' ') -coverprofile=coverage.out -covermode=atomic
                '''
            }
        }

        stage('Enforce 80% coverage gate (domain + application)') {
            steps {
                sh '''
                    go test ./internal/domain/... ./internal/application/... -coverprofile=core-coverage.out
                    go tool cover -func=core-coverage.out | tail -1 | awk '{ if ($3+0 < 80.0) { exit 1 } }'
                '''
            }
        }

        stage('Terraform fmt & validate') {
            steps {
                dir('terraform') {
                    sh '''
                        terraform fmt -check -diff
                        terraform init -backend=false
                        terraform validate
                    '''
                }
            }
        }

        stage('Validate OpenAPI contract') {
            steps {
                sh 'npx --yes @redocly/cli@2.45.0 lint api/openapi.yaml'
            }
        }

        stage('SonarCloud analysis') {
            when {
                branch 'main'
            }
            steps {
                withSonarQubeEnv('SonarCloud') {
                    sh 'sonar-scanner'
                }
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'coverage.out', allowEmptyArchive: true
        }
    }
}
