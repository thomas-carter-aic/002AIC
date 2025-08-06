terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.20"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.10"
    }
  }
  
  backend "s3" {
    bucket         = "nexus-terraform-state"
    key            = "production/terraform.tfstate"
    region         = "us-west-2"
    encrypt        = true
    dynamodb_table = "nexus-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region
  
  default_tags {
    tags = {
      Project     = "Nexus Platform"
      Environment = var.environment
      ManagedBy   = "Terraform"
      Owner       = "Platform Team"
    }
  }
}

# Data sources
data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_caller_identity" "current" {}

# Local values
locals {
  cluster_name = "nexus-${var.environment}-cluster"
  
  common_tags = {
    Project     = "Nexus Platform"
    Environment = var.environment
    ManagedBy   = "Terraform"
  }
  
  azs = slice(data.aws_availability_zones.available.names, 0, 3)
}

# VPC Configuration
module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "~> 5.0"

  name = "nexus-${var.environment}-vpc"
  cidr = var.vpc_cidr

  azs             = local.azs
  private_subnets = var.private_subnets
  public_subnets  = var.public_subnets
  database_subnets = var.database_subnets

  enable_nat_gateway = true
  enable_vpn_gateway = false
  enable_dns_hostnames = true
  enable_dns_support = true

  # Enable VPC Flow Logs
  enable_flow_log                      = true
  create_flow_log_cloudwatch_iam_role  = true
  create_flow_log_cloudwatch_log_group = true

  # Kubernetes tags
  public_subnet_tags = {
    "kubernetes.io/role/elb" = "1"
    "kubernetes.io/cluster/${local.cluster_name}" = "owned"
  }

  private_subnet_tags = {
    "kubernetes.io/role/internal-elb" = "1"
    "kubernetes.io/cluster/${local.cluster_name}" = "owned"
  }

  tags = local.common_tags
}

# EKS Cluster
module "eks" {
  source = "terraform-aws-modules/eks/aws"
  version = "~> 19.0"

  cluster_name    = local.cluster_name
  cluster_version = var.kubernetes_version

  vpc_id                         = module.vpc.vpc_id
  subnet_ids                     = module.vpc.private_subnets
  cluster_endpoint_public_access = true
  cluster_endpoint_private_access = true

  # Cluster access entry
  enable_cluster_creator_admin_permissions = true

  cluster_addons = {
    coredns = {
      most_recent = true
    }
    kube-proxy = {
      most_recent = true
    }
    vpc-cni = {
      most_recent = true
    }
    aws-ebs-csi-driver = {
      most_recent = true
    }
  }

  # EKS Managed Node Groups
  eks_managed_node_groups = {
    # Core services node group
    core = {
      name = "nexus-core-nodes"
      
      instance_types = ["m5.xlarge"]
      capacity_type  = "ON_DEMAND"
      
      min_size     = 3
      max_size     = 10
      desired_size = 6

      disk_size = 100
      disk_type = "gp3"

      labels = {
        role = "core"
        environment = var.environment
      }

      taints = []

      update_config = {
        max_unavailable_percentage = 25
      }
    }

    # AI/ML workloads node group
    ai_ml = {
      name = "nexus-ai-nodes"
      
      instance_types = ["m5.2xlarge", "c5.2xlarge"]
      capacity_type  = "SPOT"
      
      min_size     = 2
      max_size     = 20
      desired_size = 4

      disk_size = 200
      disk_type = "gp3"

      labels = {
        role = "ai-ml"
        environment = var.environment
      }

      taints = [
        {
          key    = "ai-ml"
          value  = "true"
          effect = "NO_SCHEDULE"
        }
      ]

      update_config = {
        max_unavailable_percentage = 50
      }
    }

    # GPU nodes for ML training (optional)
    gpu = {
      name = "nexus-gpu-nodes"
      
      instance_types = ["p3.2xlarge", "g4dn.xlarge"]
      capacity_type  = "SPOT"
      
      min_size     = 0
      max_size     = 5
      desired_size = 0

      disk_size = 200
      disk_type = "gp3"

      labels = {
        role = "gpu"
        environment = var.environment
        "nvidia.com/gpu" = "true"
      }

      taints = [
        {
          key    = "nvidia.com/gpu"
          value  = "true"
          effect = "NO_SCHEDULE"
        }
      ]

      update_config = {
        max_unavailable_percentage = 50
      }
    }
  }

  tags = local.common_tags
}

# RDS for PostgreSQL
module "rds" {
  source = "terraform-aws-modules/rds/aws"
  version = "~> 6.0"

  identifier = "nexus-${var.environment}-postgres"

  engine               = "postgres"
  engine_version       = "15.4"
  family               = "postgres15"
  major_engine_version = "15"
  instance_class       = var.rds_instance_class

  allocated_storage     = var.rds_allocated_storage
  max_allocated_storage = var.rds_max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true

  db_name  = "nexus"
  username = "nexus_admin"
  port     = 5432

  multi_az               = var.environment == "production"
  db_subnet_group_name   = module.vpc.database_subnet_group
  vpc_security_group_ids = [aws_security_group.rds.id]

  maintenance_window              = "Mon:00:00-Mon:03:00"
  backup_window                  = "03:00-06:00"
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
  create_cloudwatch_log_group     = true

  backup_retention_period = var.environment == "production" ? 30 : 7
  skip_final_snapshot     = var.environment != "production"
  deletion_protection     = var.environment == "production"

  performance_insights_enabled          = true
  performance_insights_retention_period = 7
  create_monitoring_role                = true
  monitoring_interval                   = 60

  tags = local.common_tags
}

# ElastiCache for Redis
module "redis" {
  source = "terraform-aws-modules/elasticache/aws"
  version = "~> 1.0"

  cluster_id           = "nexus-${var.environment}-redis"
  description          = "Nexus Platform Redis cluster"

  node_type            = var.redis_node_type
  num_cache_nodes      = var.redis_num_nodes
  parameter_group_name = "default.redis7"
  port                 = 6379
  engine_version       = "7.0"

  subnet_group_name = module.vpc.elasticache_subnet_group_name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token_enabled         = true

  apply_immediately = true
  maintenance_window = "sun:05:00-sun:09:00"

  tags = local.common_tags
}

# DocumentDB for MongoDB
resource "aws_docdb_cluster" "mongodb" {
  cluster_identifier      = "nexus-${var.environment}-mongodb"
  engine                 = "docdb"
  master_username        = "nexus_admin"
  master_password        = random_password.docdb_password.result
  backup_retention_period = var.environment == "production" ? 30 : 7
  preferred_backup_window = "07:00-09:00"
  skip_final_snapshot    = var.environment != "production"
  deletion_protection    = var.environment == "production"

  db_subnet_group_name   = aws_docdb_subnet_group.mongodb.name
  vpc_security_group_ids = [aws_security_group.docdb.id]

  storage_encrypted = true
  enabled_cloudwatch_logs_exports = ["audit", "profiler"]

  tags = local.common_tags
}

resource "aws_docdb_cluster_instance" "mongodb" {
  count              = var.docdb_instance_count
  identifier         = "nexus-${var.environment}-mongodb-${count.index}"
  cluster_identifier = aws_docdb_cluster.mongodb.id
  instance_class     = var.docdb_instance_class

  performance_insights_enabled = true

  tags = local.common_tags
}

# MSK for Kafka
module "msk" {
  source = "terraform-aws-modules/msk/aws"
  version = "~> 2.0"

  name               = "nexus-${var.environment}-kafka"
  kafka_version      = "2.8.1"
  number_of_brokers  = 3

  broker_node_client_subnets  = module.vpc.private_subnets
  broker_node_instance_type   = var.msk_instance_type
  broker_node_security_groups = [aws_security_group.msk.id]
  broker_node_storage_info = {
    ebs_storage_info = {
      volume_size = var.msk_volume_size
    }
  }

  encryption_in_transit_client_broker = "TLS"
  encryption_in_transit_in_cluster    = true

  configuration_name        = "nexus-${var.environment}-kafka-config"
  configuration_description = "Nexus Platform Kafka configuration"
  configuration_server_properties = {
    "auto.create.topics.enable" = "true"
    "default.replication.factor" = "3"
  }

  jmx_exporter_enabled    = true
  node_exporter_enabled   = true
  cloudwatch_logs_enabled = true

  tags = local.common_tags
}

# S3 Buckets for ML artifacts
module "s3_mlflow" {
  source = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 3.0"

  bucket = "nexus-${var.environment}-mlflow-artifacts-${random_id.bucket_suffix.hex}"

  # Block all public access
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true

  versioning = {
    enabled = true
  }

  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        sse_algorithm = "AES256"
      }
    }
  }

  lifecycle_configuration = {
    rule = {
      id     = "mlflow_artifacts_lifecycle"
      status = "Enabled"

      transition = [
        {
          days          = 30
          storage_class = "STANDARD_IA"
        },
        {
          days          = 90
          storage_class = "GLACIER"
        }
      ]

      expiration = {
        days = 365
      }
    }
  }

  tags = local.common_tags
}

# Application Load Balancer
module "alb" {
  source = "terraform-aws-modules/alb/aws"
  version = "~> 8.0"

  name = "nexus-${var.environment}-alb"

  load_balancer_type = "application"

  vpc_id          = module.vpc.vpc_id
  subnets         = module.vpc.public_subnets
  security_groups = [aws_security_group.alb.id]

  access_logs = {
    bucket  = module.s3_alb_logs.s3_bucket_id
    enabled = true
  }

  target_groups = [
    {
      name             = "nexus-${var.environment}-kong"
      backend_protocol = "HTTP"
      backend_port     = 80
      target_type      = "ip"
      
      health_check = {
        enabled             = true
        healthy_threshold   = 2
        interval            = 30
        matcher             = "200"
        path                = "/status"
        port                = "traffic-port"
        protocol            = "HTTP"
        timeout             = 5
        unhealthy_threshold = 2
      }
    }
  ]

  https_listeners = [
    {
      port               = 443
      protocol           = "HTTPS"
      certificate_arn    = module.acm.acm_certificate_arn
      target_group_index = 0
    }
  ]

  http_tcp_listeners = [
    {
      port        = 80
      protocol    = "HTTP"
      action_type = "redirect"
      redirect = {
        port        = "443"
        protocol    = "HTTPS"
        status_code = "HTTP_301"
      }
    }
  ]

  tags = local.common_tags
}

# ACM Certificate
module "acm" {
  source = "terraform-aws-modules/acm/aws"
  version = "~> 4.0"

  domain_name = var.domain_name
  zone_id     = data.aws_route53_zone.main.zone_id

  subject_alternative_names = [
    "*.${var.domain_name}",
    "api.${var.domain_name}",
    "admin.${var.domain_name}",
    "mlflow.${var.domain_name}",
    "grafana.${var.domain_name}",
    "airflow.${var.domain_name}"
  ]

  wait_for_validation = true

  tags = local.common_tags
}

# Route53 Records
resource "aws_route53_record" "main" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = var.domain_name
  type    = "A"

  alias {
    name                   = module.alb.lb_dns_name
    zone_id                = module.alb.lb_zone_id
    evaluate_target_health = true
  }
}

resource "aws_route53_record" "api" {
  zone_id = data.aws_route53_zone.main.zone_id
  name    = "api.${var.domain_name}"
  type    = "A"

  alias {
    name                   = module.alb.lb_dns_name
    zone_id                = module.alb.lb_zone_id
    evaluate_target_health = true
  }
}

# Security Groups
resource "aws_security_group" "alb" {
  name_prefix = "nexus-${var.environment}-alb-"
  vpc_id      = module.vpc.vpc_id

  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(local.common_tags, {
    Name = "nexus-${var.environment}-alb-sg"
  })
}

# Random resources
resource "random_password" "docdb_password" {
  length  = 16
  special = true
}

resource "random_id" "bucket_suffix" {
  byte_length = 4
}

# Data sources
data "aws_route53_zone" "main" {
  name         = var.domain_name
  private_zone = false
}
