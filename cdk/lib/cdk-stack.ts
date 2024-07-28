import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as ecs from 'aws-cdk-lib/aws-ecs';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as rds from 'aws-cdk-lib/aws-rds';
import * as elbv2 from 'aws-cdk-lib/aws-elasticloadbalancingv2';
import * as codedeploy from 'aws-cdk-lib/aws-codedeploy';
import * as secretmanager from "aws-cdk-lib/aws-secretsmanager";

export class GolangHelloWorld extends cdk.Stack {
  private readonly _vpc: ec2.Vpc;
  private readonly _cluster: ecs.Cluster;
  private readonly _securityGroup: ec2.SecurityGroup;
  private readonly _loadBalancer: elbv2.ApplicationLoadBalancer;
  private readonly _blueTargetGroup: elbv2.ApplicationTargetGroup;
  private readonly _greenTargetGroup: elbv2.ApplicationTargetGroup;
  private readonly _listener: elbv2.ApplicationListener;
  private readonly _testListener: elbv2.ApplicationListener;
  private readonly _dbCredentialsSecret: secretmanager.Secret;
  private readonly _dbSecurityGroup: ec2.SecurityGroup;
  private readonly _dbSubnetGroup: rds.SubnetGroup;
  private readonly _dbCluster: rds.ServerlessCluster;
  private readonly _rdsParameterGroup: rds.ParameterGroup;
  private readonly _ecrRepository: ecr.Repository;
  private readonly _codeDeployApp: codedeploy.EcsApplication;
  private readonly _deploymentGroup: codedeploy.EcsDeploymentGroup;
  private readonly _ecsService: ecs.FargateService;

  constructor(scope: cdk.App, id: string, props?: cdk.StackProps) {
    super(scope, id, props);

    // =========== VPC ===========

    this._vpc = new ec2.Vpc(this, "CustomVpc", {
      subnetConfiguration: [
        {
          name: "custom-vpc-public-subnet",
          subnetType: ec2.SubnetType.PUBLIC,
          cidrMask: 24,
        },
        {
          name: "custom-vpc-private-subnet",
          subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
          cidrMask: 24,
        },
        {
          name: "custom-vpc-isolated-subnet",
          subnetType: ec2.SubnetType.PRIVATE_ISOLATED,
          cidrMask: 24,
        },
      ],
      maxAzs: 2,
      natGateways: 2,
      vpcName: "CustomVpc",
    });

    this._vpc.addInterfaceEndpoint("EcrEndpoint", {
      service: ec2.InterfaceVpcEndpointAwsService.ECR,
    });


    // =========== RDS ===========

    // Create a secret for the RDS credentials
    this._dbCredentialsSecret = new secretmanager.Secret(this, 'DBCredentialsSecret', {
      secretName: 'postgresCredentials',
      generateSecretString: {
        secretStringTemplate: JSON.stringify({username: 'golang'}),
        excludePunctuation: true,
        includeSpace: false,
        generateStringKey: 'password',
        passwordLength: 24,
      },
    });

    // Create Security Group for the RDS cluster
    this._dbSecurityGroup = new ec2.SecurityGroup(this, 'DBSecurityGroup', {
      vpc: this._vpc,
      description: 'Allow connections to Aurora PostgreSQL',
      securityGroupName: 'DBSecurityGroup',
      allowAllOutbound: true,
    });

    // Add RDS inbound rules for service to be able to access the database
    this._dbSecurityGroup.addIngressRule(ec2.Peer.ipv4(this._vpc.vpcCidrBlock), ec2.Port.tcp(5432), 'Allow inbound from VPC');

    // RDS Subnet Group
    this._dbSubnetGroup = new rds.SubnetGroup(this, 'DBSubnetGroup', {
      vpc: this._vpc,
      description: 'Database subnet group',
      vpcSubnets: this._vpc.selectSubnets({subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS}),
      subnetGroupName: 'DBSubnetGroup',
    });

    // RDS Parameter Group

    const rdsEngine = rds.DatabaseClusterEngine.auroraPostgres({version: rds.AuroraPostgresEngineVersion.VER_13_14});

    this._rdsParameterGroup = new rds.ParameterGroup(this, 'RDSParameterGroup', {
      engine: rdsEngine,
      parameters: {
        'rds.force_ssl': '1',
      },
    });


    // Create the RDS serverless cluster
    this._dbCluster = new rds.ServerlessCluster(this, 'ServerlessPostgresCluster', {
      engine: rdsEngine,
      vpc: this._vpc,
      credentials: rds.Credentials.fromSecret(this._dbCredentialsSecret),
      scaling: {
        autoPause: cdk.Duration.minutes(10), // Auto pause after 10 minutes of inactivity
        minCapacity: rds.AuroraCapacityUnit.ACU_2, // Minimum capacity
        maxCapacity: rds.AuroraCapacityUnit.ACU_4, // Maximum capacity
      },
      parameterGroup: this._rdsParameterGroup,
      defaultDatabaseName: 'hello',
      removalPolicy: cdk.RemovalPolicy.DESTROY, // in production this would be RETAIN
      clusterIdentifier: 'golang-hello-cluster',
      backupRetention: cdk.Duration.days(7),
      enableDataApi: true,
      securityGroups: [this._dbSecurityGroup],
      subnetGroup: this._dbSubnetGroup,
    });

    // =========== ECS ===========

    this._cluster = new ecs.Cluster(this, 'EcsCluster', {
      clusterName: 'hello-cluster',
      vpc: this._vpc,
    });

    // =========== ECS Load Balancer =

    this._securityGroup = new ec2.SecurityGroup(this, 'SecurityGroup', {
      vpc: this._vpc,
      allowAllOutbound: true
    })

    this._securityGroup.addIngressRule(this._securityGroup, ec2.Port.tcp(8080), 'Group Inbound', false);

    this._loadBalancer = new elbv2.ApplicationLoadBalancer(this, 'NetworkLoadBalancer', {
      vpc: this._vpc,
      loadBalancerName: 'hello-cluster-nlb',
      vpcSubnets: {
        subnets: this._vpc.privateSubnets,
        onePerAz: true,
        availabilityZones: this._vpc.availabilityZones
      },
      securityGroup: this._securityGroup
    });

    // =========== ECS Target Groups ===========

    this._blueTargetGroup = new elbv2.ApplicationTargetGroup(this, 'blueGroup', {
      vpc: this._vpc,
      port: 80,
      targetGroupName: "hello-cluster-blue",
      targetType: elbv2.TargetType.IP,
      healthCheck: {
        protocol: elbv2.Protocol.HTTP,
        path: '/health',
        timeout: cdk.Duration.seconds(30),
        interval: cdk.Duration.seconds(60),
        healthyHttpCodes: '200'
      }
    });

    this._greenTargetGroup = new elbv2.ApplicationTargetGroup(this, 'greenGroup', {
      vpc: this._vpc,
      port: 80,
      targetType: elbv2.TargetType.IP,
      targetGroupName: "hello-cluster-green",
      healthCheck: {
        protocol: elbv2.Protocol.HTTP,
        path: '/health',
        timeout: cdk.Duration.seconds(30),
        interval: cdk.Duration.seconds(60),
        healthyHttpCodes: '200'
      }
    });

    this._listener = this._loadBalancer.addListener('albProdListener', {
      port: 80,
      defaultTargetGroups: [this._blueTargetGroup]
    });

    this._testListener = this._loadBalancer.addListener('albTestListener', {
      port: 8080,
      defaultTargetGroups: [this._greenTargetGroup]
    });

    // ======== ECR Repository =========

    this._ecrRepository = new ecr.Repository(this, 'EcrRepository', {
      repositoryName: 'volaka/golang-hello',
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      imageTagMutability: ecr.TagMutability.IMMUTABLE,
      lifecycleRules: [
        {
          maxImageCount: 8,
          description: 'Keep only the latest 8 images'
        }
      ]
    });


    // =========== ECS Service ===========

    const taskDefinition = new ecs.FargateTaskDefinition(this, 'TaskDef', {
      cpu: 256,
      memoryLimitMiB: 512,
    });

    const container = taskDefinition.addContainer('AppContainer', {
      image: ecs.ContainerImage.fromRegistry(process.env.ECR_IMAGE_URI),
      memoryLimitMiB: 512,
      cpu: 256,
      essential: true,
      logging: ecs.LogDrivers.awsLogs({ streamPrefix: 'ecs' }),
    });

    container.addPortMappings({
      containerPort: 80,
    });

    this._ecsService = new ecs.FargateService(this, 'EcsService', {
      cluster: this._cluster,
      taskDefinition,
      desiredCount: 1,
      securityGroups: [this._securityGroup],
      vpcSubnets: {
        subnets: this._vpc.privateSubnets,
      },
    });

    // Define CodeDeploy Application
    this._codeDeployApp = new codedeploy.EcsApplication(this, 'CodeDeployApp', {
      applicationName: 'GolangHelloCodeDeployApp',
    });

    // Define CodeDeploy Deployment Group
    this._deploymentGroup = new codedeploy.EcsDeploymentGroup(this, 'DeploymentGroup', {
      application: this._codeDeployApp,
      deploymentGroupName: 'GolangHelloDeploymentGroup',
      service: ecs.FargateService.fromFargateServiceAttributes(this, 'FargateService', {
        cluster: this._cluster,
        serviceName: 'hello-service',
      }),
      blueGreenDeploymentConfig: {
        blueTargetGroup: this._blueTargetGroup,
        greenTargetGroup: this._greenTargetGroup,
        listener: this._listener,
        testListener: this._testListener,
      },
      autoRollback: {
        failedDeployment: true,
        stoppedDeployment: true,
      },
    });

    // ========== OUTPUTS ============

    // Output ALB DNS name
    new cdk.CfnOutput(this, 'ALBDnsName', {
      value: this._loadBalancer.loadBalancerDnsName,
      exportName: 'ALBDnsName',
    });

    // Output Blue Target Group ARN
    new cdk.CfnOutput(this, 'BlueTargetGroupArn', {
      value: this._blueTargetGroup.targetGroupArn,
      exportName: 'BlueTargetGroupArn',
    });

    // Output Green Target Group ARN
    new cdk.CfnOutput(this, 'GreenTargetGroupArn', {
      value: this._greenTargetGroup.targetGroupArn,
      exportName: 'GreenTargetGroupArn',
    });

    // Output CodeDeploy Application Name
    new cdk.CfnOutput(this, 'CodeDeployAppName', {
      value: this._codeDeployApp.applicationName,
      exportName: 'CodeDeployAppName',
    });

    // Output CodeDeploy Deployment Group Name
    new cdk.CfnOutput(this, 'CodeDeployDeploymentGroupName', {
      value: this._deploymentGroup.deploymentGroupName,
      exportName: 'CodeDeployDeploymentGroupName',
    });
  }
}