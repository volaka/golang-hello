#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -ex

# Retrieve stack outputs
STACK_NAME="GolangHello"
ALB_DNS_NAME=$(aws cloudformation describe-stacks --stack-name $STACK_NAME --query "Stacks[0].Outputs[?OutputKey=='ALBDnsName'].OutputValue" --output text)
BLUE_TARGET_GROUP_ARN=$(aws cloudformation describe-stacks --stack-name $STACK_NAME --query "Stacks[0].Outputs[?OutputKey=='BlueTargetGroupArn'].OutputValue" --output text)
GREEN_TARGET_GROUP_ARN=$(aws cloudformation describe-stacks --stack-name $STACK_NAME --query "Stacks[0].Outputs[?OutputKey=='GreenTargetGroupArn'].OutputValue" --output text)

# Perform blue-green deployment using AWS CLI
aws deploy create-deployment \
  --application-name CodeDeployApp \
  --deployment-group-name DeploymentGroup \
  --deployment-config-name CodeDeployDefault.ECSAllAtOnce \
  --target-group-pair-info "targetGroups=[{name=$BLUE_TARGET_GROUP_ARN},{name=$GREEN_TARGET_GROUP_ARN}]" \
  --load-balancer-info "elbInfoList=[{name=$ALB_DNS_NAME}]"

echo "Blue-green deployment initiated."