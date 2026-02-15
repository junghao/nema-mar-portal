variable "environment" {
  description = "Deployment environment (dev, prod)"
  type        = string
  default     = "dev"
}

variable "region" {
  description = "AWS region"
  type        = string
  default     = "ap-southeast-2"
}

variable "docker_tag" {
  description = "Docker image tag for nema-mar-app"
  type        = string
}

variable "ecs_cluster_id" {
  description = "ECS cluster ID"
  type        = string
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "private_subnet_ids" {
  description = "Private subnet IDs for RDS and ECS"
  type        = list(string)
}

variable "public_subnet_ids" {
  description = "Public subnet IDs for ALB"
  type        = list(string)
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_version" {
  description = "PostgreSQL engine version"
  type        = string
  default     = "16.4"
}

variable "db_username" {
  description = "Database master username"
  type        = string
  default     = "fastschema"
  sensitive   = true
}

variable "db_password" {
  description = "Database master password"
  type        = string
  sensitive   = true
}

variable "db_subnet_group_name" {
  description = "RDS subnet group name"
  type        = string
}

variable "db_security_group_ids" {
  description = "Security group IDs for RDS"
  type        = list(string)
}

variable "storage_bucket" {
  description = "S3 bucket for FastSchema object store"
  type        = string
}

variable "fastschema_app_key" {
  description = "FastSchema APP_KEY (32 character string)"
  type        = string
  sensitive   = true
}

variable "smtp_host" {
  description = "SMTP server host"
  type        = string
  default     = ""
}

variable "smtp_port" {
  description = "SMTP server port"
  type        = string
  default     = "587"
}

variable "smtp_from" {
  description = "SMTP sender address"
  type        = string
  default     = ""
}

variable "ddog_api_key" {
  description = "DataDog API key"
  type        = string
  default     = ""
  sensitive   = true
}

variable "log_retention" {
  description = "CloudWatch log retention in days"
  type        = number
  default     = 30
}

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS"
  type        = string
  default     = ""
}
