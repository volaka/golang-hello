import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as rds from 'aws-cdk-lib/aws-rds';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';

export class GolangHello extends cdk.Stack {
  constructor(scope: Construct, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // Create a VPC
    const vpc = new ec2.Vpc(this, 'HelloVPC', {
      maxAzs: 3
    });

    // Create an ECS cluster
    const cluster = new ecs.Cluster(this, 'HelloCluster', {
      vpc: vpc,
      clusterName: 'golang-hello-cluster'
    });

    // Create an ECR repository
    const repository = new ecr.Repository(this, 'HelloRepository', {
      repositoryName: 'golang-hello',
      imageTagMutability: ecr.TagMutability.MUTABLE,
      lifecycleRules: [{
        maxImageCount: 8,
      }],
    });

    // Define the ECS Fargate task definition
    const taskDefinition = new ecs.FargateTaskDefinition(this, 'HelloTaskDefinition', {
      memoryLimitMiB: 512,
      cpu: 256,
    });

    // Create a secret for the RDS credentials
    const dbCredentialsSecret = new secretsmanager.Secret(this, 'DBCredentialsSecret', {
      secretName: 'postgresCredentials',
      generateSecretString: {
        secretStringTemplate: JSON.stringify({ username: 'golang' }),
        excludePunctuation: true,
        includeSpace: false,
        generateStringKey: 'password',
        passwordLength: 24,
      },
    });

    // Create Security Group for the RDS cluster
    const dbSecurityGroup = new ec2.SecurityGroup(this, 'DBSecurityGroup', {
      vpc,
      description: 'Allow connections to Aurora PostgreSQL',
      securityGroupName: 'DBSecurityGroup',
      allowAllOutbound: true,
    });

    // Add RDS inbound rules for service to be able to access the database
    dbSecurityGroup.addIngressRule(ec2.Peer.ipv4(vpc.vpcCidrBlock), ec2.Port.tcp(5432), 'Allow inbound from VPC');

    // RDS Subnet Group
    const dbSubnetGroup = new rds.SubnetGroup(this, 'DBSubnetGroup', {
      vpc,
      description: 'Database subnet group',
      vpcSubnets: vpc.selectSubnets({ subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS }),
      subnetGroupName: 'DBSubnetGroup',
    });

    // Create the RDS serverless cluster
    const dbCluster = new rds.ServerlessCluster(this, 'ServerlessPostgresCluster', {
      engine: rds.DatabaseClusterEngine.AURORA_POSTGRESQL,
      vpc,
      credentials: rds.Credentials.fromSecret(dbCredentialsSecret),
      scaling: {
        autoPause: cdk.Duration.minutes(10), // Auto pause after 10 minutes of inactivity
        minCapacity: rds.AuroraCapacityUnit.ACU_1, // Minimum capacity
        maxCapacity: rds.AuroraCapacityUnit.ACU_4, // Maximum capacity
      },
      defaultDatabaseName: 'golang-hello',
      removalPolicy: cdk.RemovalPolicy.RETAIN,
      clusterIdentifier: 'golang-hello-cluster',
      backupRetention: cdk.Duration.days(7),
      enableDataApi: true,
      securityGroups: [dbSecurityGroup],
      subnetGroup: dbSubnetGroup,
    });

    // Add a container to the task definition
    const container = taskDefinition.addContainer('HelloContainer', {
      image: ecs.ContainerImage.fromEcrRepository(repository),
      logging: ecs.LogDrivers.awsLogs({ streamPrefix: 'golang-hello' }),
      environment: {
        DB_HOST: dbCluster.clusterEndpoint.hostname,
        DB_NAME: 'golang-hello',
        DB_USER: dbCredentialsSecret.secretValueFromJson('username').toString(),
        DB_PASSWORD: dbCredentialsSecret.secretValueFromJson('password').toString(),
      },
    });

    container.addPortMappings({
      containerPort: 8080,
    });

    // Create ECS Fargate service
    const service = new ecs.FargateService(this, 'HelloService', {
      cluster: cluster,
      taskDefinition: taskDefinition,
      desiredCount: 1,
    });

    // Create an Application Load Balancer (ALB)
    const alb = new elbv2.ApplicationLoadBalancer(this, 'ALB', {
      vpc: vpc,
      internetFacing: true,
    });

    // Create a target group for the service
    const targetGroup = new elbv2.ApplicationTargetGroup(this, 'TargetGroup', {
      vpc: vpc,
      port: 8080,
      protocol: elbv2.ApplicationProtocol.HTTP,
      targets: [service],
    });

    // Create a listener for the ALB
    const listener = alb.addListener('Listener', {
      port: 80,
      defaultTargetGroups: [targetGroup],
    });
  }
}