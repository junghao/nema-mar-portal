terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_caller_identity" "current" {}

locals {
  project_name = "nema-mar"
  name_prefix  = "tf-${var.environment}-${local.project_name}"
}

# --- PostgreSQL for FastSchema ---

resource "aws_db_instance" "fastschema_db" {
  identifier             = "${local.name_prefix}-fastschema"
  allocated_storage      = 20
  storage_type           = "gp3"
  engine                 = "postgres"
  engine_version         = var.db_version
  instance_class         = var.db_instance_class
  db_name                = "fastschema"
  username               = var.db_username
  password               = var.db_password
  db_subnet_group_name   = var.db_subnet_group_name
  publicly_accessible    = false
  multi_az               = var.environment == "prod"
  skip_final_snapshot    = var.environment != "prod"
  vpc_security_group_ids = var.db_security_group_ids
  deletion_protection    = var.environment == "prod"

  tags = {
    environment           = var.environment
    "gns:billing:project" = local.project_name
  }
}

# --- S3 prefix for FastSchema object store ---
# The bucket is assumed to already exist; FastSchema uses a prefix within it.

# --- SSM Parameters for secrets ---

resource "aws_ssm_parameter" "smtp_username" {
  name  = "/${local.name_prefix}/smtp-username"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "smtp_password" {
  name  = "/${local.name_prefix}/smtp-password"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "smtp_recipients" {
  name  = "/${local.name_prefix}/smtp-recipients"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "fs_admin_user" {
  name  = "/${local.name_prefix}/fs-admin-user"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

resource "aws_ssm_parameter" "fs_admin_pass" {
  name  = "/${local.name_prefix}/fs-admin-pass"
  type  = "SecureString"
  value = "placeholder"

  lifecycle {
    ignore_changes = [value]
  }
}

# --- IAM Roles ---

resource "aws_iam_role" "task_role" {
  name = "${local.name_prefix}-task-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy" "task_s3" {
  name = "${local.name_prefix}-task-s3"
  role = aws_iam_role.task_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject",
        "s3:ListBucket"
      ]
      Resource = [
        "arn:aws:s3:::${var.storage_bucket}",
        "arn:aws:s3:::${var.storage_bucket}/${local.project_name}/*"
      ]
    }]
  })
}

resource "aws_iam_role" "execution_role" {
  name = "${local.name_prefix}-execution-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "execution_ecs" {
  role       = aws_iam_role.execution_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "execution_ssm" {
  name = "${local.name_prefix}-execution-ssm"
  role = aws_iam_role.execution_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "ssm:GetParameters",
        "ssm:GetParameter"
      ]
      Resource = "arn:aws:ssm:${var.region}:${data.aws_caller_identity.current.account_id}:parameter/${local.name_prefix}/*"
    }]
  })
}

# --- CloudWatch Log Groups ---

resource "aws_cloudwatch_log_group" "app" {
  name              = "${local.name_prefix}-portal"
  retention_in_days = var.log_retention
}

resource "aws_cloudwatch_log_group" "fastschema" {
  name              = "${local.name_prefix}-fastschema"
  retention_in_days = var.log_retention
}

# --- ECS Task Definition (sidecar pattern) ---

resource "aws_ecs_task_definition" "nema_mar_portal" {
  family                = "${local.name_prefix}-portal"
  network_mode          = "bridge"
  task_role_arn         = aws_iam_role.task_role.arn
  execution_role_arn    = aws_iam_role.execution_role.arn

  container_definitions = jsonencode([
    {
      name      = "nema-mar-app"
      image     = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${var.region}.amazonaws.com/nema-mar-app:${var.docker_tag}"
      essential = true
      memory    = 256
      cpu       = 128
      portMappings = [{ containerPort = 8080, hostPort = 0, protocol = "tcp" }]
      environment = [
        { name = "FASTSCHEMA_URL", value = "http://localhost:8000" },
        { name = "SMTP_HOST",      value = var.smtp_host },
        { name = "SMTP_PORT",      value = var.smtp_port },
        { name = "SMTP_FROM",      value = var.smtp_from },
        { name = "DDOG_API_KEY",   value = var.ddog_api_key },
      ]
      secrets = [
        { name = "SMTP_USERNAME",   valueFrom = aws_ssm_parameter.smtp_username.arn },
        { name = "SMTP_PASSWORD",   valueFrom = aws_ssm_parameter.smtp_password.arn },
        { name = "SMTP_RECIPIENTS", valueFrom = aws_ssm_parameter.smtp_recipients.arn },
        { name = "FS_ADMIN_USER",   valueFrom = aws_ssm_parameter.fs_admin_user.arn },
        { name = "FS_ADMIN_PASS",   valueFrom = aws_ssm_parameter.fs_admin_pass.arn },
      ]
      dependsOn = [{ containerName = "fastschema", condition = "HEALTHY" }]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"  = aws_cloudwatch_log_group.app.name
          "awslogs-region" = var.region
        }
      }
      healthCheck = {
        command     = ["CMD", "/usr/local/bin/nema-mar-app", "-check"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 10
      }
    },
    {
      name      = "fastschema"
      image     = "ghcr.io/fastschema/fastschema:latest"
      essential = true
      memory    = 256
      cpu       = 128
      # Not exposed to host — only reachable via localhost within the task
      portMappings = []
      environment = [
        { name = "APP_KEY",   value = var.fastschema_app_key },
        { name = "APP_PORT",  value = "8000" },
        { name = "DB_DRIVER", value = "pgx" },
        { name = "DB_HOST",   value = aws_db_instance.fastschema_db.address },
        { name = "DB_PORT",   value = "5432" },
        { name = "DB_NAME",   value = "fastschema" },
        { name = "DB_USER",   value = var.db_username },
        { name = "DB_PASS",   value = var.db_password },
        {
          name  = "STORAGE"
          value = jsonencode({
            default_disk = "s3"
            disks = [{
              name        = "s3"
              driver      = "s3"
              root        = "${local.project_name}/files"
              bucket      = var.storage_bucket
              region      = var.region
              base_url    = "https://${var.storage_bucket}.s3.${var.region}.amazonaws.com"
              public_path = "/files"
            }]
          })
        },
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"  = aws_cloudwatch_log_group.fastschema.name
          "awslogs-region" = var.region
        }
      }
      healthCheck = {
        command     = ["CMD-SHELL", "wget -qO- http://localhost:8000/dash || exit 1"]
        interval    = 15
        timeout     = 5
        retries     = 5
        startPeriod = 30
      }
    }
  ])
}

# --- ALB ---

resource "aws_lb" "lb" {
  name               = "${local.name_prefix}-alb"
  internal           = false
  load_balancer_type = "application"
  subnets            = var.public_subnet_ids

  tags = {
    environment           = var.environment
    "gns:billing:project" = local.project_name
  }
}

resource "aws_lb_target_group" "tg" {
  name     = "${local.name_prefix}-tg"
  port     = 8080
  protocol = "HTTP"
  vpc_id   = var.vpc_id

  health_check {
    path                = "/soh"
    healthy_threshold   = 2
    unhealthy_threshold = 5
    interval            = 30
    timeout             = 5
  }
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.lb.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.tg.arn
  }
}

# --- ECS Service ---

resource "aws_ecs_service" "nema_mar_portal" {
  name            = "${local.name_prefix}-portal"
  cluster         = var.ecs_cluster_id
  task_definition = aws_ecs_task_definition.nema_mar_portal.arn
  desired_count   = var.environment == "prod" ? 2 : 1

  load_balancer {
    target_group_arn = aws_lb_target_group.tg.arn
    container_name   = "nema-mar-app"
    container_port   = 8080
  }
}
