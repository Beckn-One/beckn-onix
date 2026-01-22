import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as helm from 'aws-cdk-lib/aws-eks';
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { ConfigProps } from './config';

interface HelmGatewayStackProps extends cdk.StackProps {
  config: ConfigProps;
  eksCluster: eks.Cluster;
}

/**
 * Deploys the beckn-onix-gateway Helm chart.
 *
 * NOTE: This stack no longer depends on Aurora/Postgres. The gateway
 * now uses an embedded H2 database on its own persistent volume, as
 * configured in the Helm chart's swf.properties and PVC templates.
 */
export class HelmGatewayStack extends Stack {
  constructor(scope: Construct, id: string, props: HelmGatewayStackProps) {
    super(scope, id, props);

    const eksCluster = props.eksCluster;
    const externalDomain = props.config.GATEWAY_EXTERNAL_DOMAIN;
    const certArn = props.config.CERT_ARN;
    const registryUrl = props.config.REGISTRY_URL;

    const releaseName = props.config.GATEWAY_RELEASE_NAME;
    const repository = props.config.REPOSITORY;

    new helm.HelmChart(this, 'gatewayhelm', {
      cluster: eksCluster,
      chart: 'beckn-onix-gateway',
      release: releaseName,
      wait: false,
      repository: repository,
      values: {
        externalDomain: externalDomain,
        registry_url: registryUrl,
        ingress: {
          tls: {
            certificateArn: certArn,
          },
        },
      },
    });
  }
}
