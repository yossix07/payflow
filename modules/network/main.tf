# =============================================================================
# NETWORK MODULE - MAIN CONFIGURATION
# =============================================================================
# This module builds the fundamental networking layer for AWS.
#
# ARCHITECTURE OVERVIEW:
# 1. VPC: The private network container.
# 2. Public Subnets: Connected to Internet Gateway (IGW). For Load Balancers.
# 3. Private Subnets: Connected to NAT Gateway. For Application Nodes.
#     - Private means: NO direct access FROM the internet (Safe).
#     - NAT means: Can access internet OUTBOUND (Updates/API calls).
# =============================================================================

# =============================================================================
# 1. VPC (Virtual Private Cloud)
# =============================================================================
resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr

  # Enable DNS hostnames - required for EKS to work properly!
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "${var.env}-main-vpc"
    Environment = var.env
  }
}

# =============================================================================
# 2. INTERNET GATEWAY (IGW)
# =============================================================================
# Like the modem in your house - connects the VPC to the outside world.
# Only PUBLIC subnets will route traffic to this.
resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name        = "${var.env}-igw"
    Environment = var.env
  }
}

# =============================================================================
# 3. PUBLIC SUBNETS
# =============================================================================
# Used for: Load Balancers (Ingress), Nat Gateways, Bastion Hosts.
# 
# "count" loop: Creates one subnet for each element in var.public_subnets
resource "aws_subnet" "public" {
  count = length(var.public_subnets)

  vpc_id            = aws_vpc.main.id
  cidr_block        = var.public_subnets[count.index]
  availability_zone = var.azs[count.index]

  # Important: Auto-assign public IPs to instances launched here
  map_public_ip_on_launch = true

  tags = {
    Name = "${var.env}-public-${var.azs[count.index]}"
    
    # KUBERNETES TAGGING REQUIREMENT:
    # EKS needs these tags to know where to place Load Balancers (ELB)
    "kubernetes.io/role/elb" = "1"
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# =============================================================================
# 4. PRIVATE SUBNETS
# =============================================================================
# Used for: EKS Worker Nodes, Databases.
# These nodes have NO public IP addresses.
resource "aws_subnet" "private" {
  count = length(var.private_subnets)

  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnets[count.index]
  availability_zone = var.azs[count.index]

  tags = {
    Name = "${var.env}-private-${var.azs[count.index]}"
    
    # KUBERNETES TAGGING REQUIREMENT:
    # EKS needs these tags to know where to place Internal Load Balancers
    "kubernetes.io/role/internal-elb" = "1"
    "kubernetes.io/cluster/${var.cluster_name}" = "shared"
  }
}

# =============================================================================
# 5. NAT GATEWAY (The Costly Part)
# =============================================================================
# Allows private instances to initiate outbound traffic to the internet
# (e.g., for software updates) but prevents internet from connecting IN.
#
# Requires:
#   1. An Elastic IP (Static Public IP)
#   2. To be placed in a PUBLIC subnet
#
# COST SAVING NOTE:
# For production, we usually want one NAT Gateway per AZ (High Availability).
# For dev/learning, we create just ONE shared NAT Gateway to save money.
# (~$32/month vs ~$64/month for 2 AZs).
resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  
  # Put NAT in the FIRST public subnet
  subnet_id     = aws_subnet.public[0].id

  tags = {
    Name        = "${var.env}-nat"
    Environment = var.env
  }

  # Terraform Dependency: Wait for IGW to be ready
  depends_on = [aws_internet_gateway.main]
}

# =============================================================================
# 6. ROUTE TABLES (The Traffic Cops)
# =============================================================================

# PUBLIC ROUTE TABLE
# Rule: "If traffic is destined for internet (0.0.0.0/0), send to IGW"
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }

  tags = {
    Name = "${var.env}-public-rt"
  }
}

# PRIVATE ROUTE TABLE
# Rule: "If traffic is destined for internet, send to NAT Gateway"
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }

  tags = {
    Name = "${var.env}-private-rt"
  }
}

# =============================================================================
# 7. ROUTE TABLE ASSOCIATIONS
# =============================================================================
# Connect the Subnets to the Route Tables.

# Associate ALL Public subnets with Public Route Table
resource "aws_route_table_association" "public" {
  count = length(var.public_subnets)

  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Associate ALL Private subnets with Private Route Table
resource "aws_route_table_association" "private" {
  count = length(var.private_subnets)

  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}
