import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as helm from 'aws-cdk-lib/aws-eks';
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { ConfigProps } from './config';
import * as crypto from 'crypto';

interface HelmCommonServicesStackProps extends StackProps {
    config: ConfigProps;
    eksCluster: eks.Cluster;
    service: string,
}

export class HelmCommonServicesStack extends Stack {
    constructor(scope: Construct, id: string, props: HelmCommonServicesStackProps) {
        super(scope, id, props);
        
        const eksCluster = props.eksCluster;
        const service = props.service;
        const repository = "https://charts.bitnami.com/bitnami";
        const namespace = props.config.NAMESPACE;

        // Redis - Using Amazon ECR Public
        new helm.HelmChart(this, "RedisHelmChart", {
            cluster: eksCluster,
            chart: "redis",
            namespace: service + namespace,
            release: "redis",
            wait: false,
            repository: repository,
            version: "19.5.5",
            values: {
                image: {
                    registry: "public.ecr.aws",
                    repository: "bitnami/redis",
                    tag: "7.2.5"
                },
                auth: {
                    enabled: false
                },
                replica: {
                    replicaCount: 0
                },
                master: {
                    persistence: {
                        storageClass: "gp2"
                    }
                }
            }
        });

        // RabbitMQ - Using Amazon ECR Public
        new helm.HelmChart(this, "RabbitMQHelmChart", {
            cluster: eksCluster,
            chart: "rabbitmq",
            namespace: service + namespace,
            release: "rabbitmq",
            wait: false,
            repository: repository,
            version: "15.0.1",
            values: {
                image: {
                    registry: "public.ecr.aws",
                    repository: "bitnami/rabbitmq",
                    tag: "3.13.7"
                },
                persistence: {
                    enabled: true,
                    storageClass: "gp2"
                },
                auth: {
                    username: "beckn",
                    password: "beckn1234"
                }
            }
        });
    }
}