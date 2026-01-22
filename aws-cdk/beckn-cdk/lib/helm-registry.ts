import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as helm from 'aws-cdk-lib/aws-eks';
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { ConfigProps } from './config';

interface HelmRegistryStackProps extends StackProps {
  config: ConfigProps;
  eksCluster: eks.Cluster;
}

/**
 * Deploys the beckn-onix-registry Helm chart.
 *
 * NOTE: This stack no longer depends on Aurora/Postgres. The registry
 * now uses an embedded H2 database on its own persistent volume, as
 * configured in the Helm chart's swf.properties and PVC templates.
 */
export class HelmRegistryStack extends Stack {
  constructor(scope: Construct, id: string, props: HelmRegistryStackProps) {
    super(scope, id, props);

    const eksCluster = props.eksCluster;
    const externalDomain = props.config.REGISTRY_EXTERNAL_DOMAIN;
    const certArn = props.config.CERT_ARN;
    const releaseName = props.config.REGISTRY_RELEASE_NAME;
    const repository = props.config.REPOSITORY;

    new helm.HelmChart(this, 'registryhelm', {
      cluster: eksCluster,
      chart: 'beckn-onix-registry',
      release: releaseName,
      wait: false,
      repository: repository,
      values: {
        externalDomain: externalDomain,
        ingress: {
          tls: {
            certificateArn: certArn,
          },
        },
      },
    });
  }
}
