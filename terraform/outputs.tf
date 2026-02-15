output "alb_dns_name" {
  description = "ALB DNS name"
  value       = aws_lb.lb.dns_name
}

output "rds_endpoint" {
  description = "RDS endpoint for FastSchema"
  value       = aws_db_instance.fastschema_db.endpoint
}

output "ecs_service_name" {
  description = "ECS service name"
  value       = aws_ecs_service.nema_mar_portal.name
}
