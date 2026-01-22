import * as cdk from 'aws-cdk-lib';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as helm from 'aws-cdk-lib/aws-eks';
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { ConfigProps } from './config';
import * as ec2 from 'aws-cdk-lib/aws-ec2';


interface HelmBapStackProps extends StackProps {
  config: ConfigProps;
  eksCluster: eks.Cluster;
  isSandbox: boolean;
  eksSecGrp: ec2.SecurityGroup;
  vpc: ec2.Vpc;
}

export class HelmBapStack extends Stack {
  constructor(scope: Construct, id: string, props: HelmBapStackProps) {
    super(scope, id, props);

    const eksCluster = props.eksCluster;
    const externalDomain = props.config.BAP_EXTERNAL_DOMAIN;
    const certArn = props.config.CERT_ARN;
    const releaseName = props.config.BAP_RELEASE_NAME;
    const repository = props.config.REPOSITORY;
    const registryUrl = props.config.REGISTRY_URL;
    const bapPrivateKey = props.config.BAP_PRIVATE_KEY;
    const bapPublicKey = props.config.BAP_PUBLIC_KEY;

    const isSandbox = props.isSandbox;

    
    new helm.HelmChart(this, 'baphelm', {
      cluster: eksCluster,
      chart: 'beckn-onix-bap',
      release: releaseName,
      wait: false,
      repository: repository,
      values: {
        global: {
          isSandbox: isSandbox,
          externalDomain: externalDomain,
          registry_url: registryUrl,
          bap: {
            privateKey: bapPrivateKey,
            publicKey: bapPublicKey,
          },
          ingress: {
            tls: {
              certificateArn: certArn,
          },
        },
      },
    },
  }
);
    
  }
}
