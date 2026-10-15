import type { TokenKind } from './clowd'

// examples/app.clowd, split into the statements that produce resources, with
// the AWS resources each one became in `clowd build --target aws`
// (src/data/app.aws.tf.json). Every one of the 37 resources appears once.
export type Step = { src: string; tint: TokenKind; resources: string[] }

export const STEPS: Step[] = [
  {
    src: 'use aws in eu-west-2\n',
    tint: 'cloud',
    resources: [
      'aws_vpc.main',
      'aws_internet_gateway.main',
      'aws_subnet.public',
      'aws_subnet.private',
      'aws_route_table.public',
      'aws_route_table.private',
      'aws_route_table_association.public',
      'aws_route_table_association.private',
      'aws_route.public_internet',
      'aws_ecs_cluster.main',
    ],
  },
  { src: 'use gcp in europe-west2\n\n', tint: 'cloud', resources: [] },
  {
    src: 'database db is postgres 16, small\n  on aws\n    backup_retention_period 14\n\n',
    tint: 'kind',
    resources: ['aws_db_subnet_group.db', 'aws_security_group.db', 'aws_db_instance.db'],
  },
  {
    src: 'bucket uploads is private\n\n',
    tint: 'trait',
    resources: ['aws_s3_bucket.uploads', 'aws_s3_bucket_public_access_block.uploads'],
  },
  {
    src: 'service api\n  image ghcr.io/me/api:latest\n  port 8080\n  scale 2 to 10\n\n',
    tint: 'prop',
    resources: [
      'aws_security_group.api',
      'aws_cloudwatch_log_group.api',
      'aws_iam_role.api_execution',
      'aws_iam_role.api_task',
      'aws_iam_role_policy_attachment.api_execution',
      'aws_vpc_security_group_egress_rule.api_all',
      'aws_ecs_task_definition.api',
      'aws_ecs_service.api',
      'aws_appautoscaling_target.api',
      'aws_appautoscaling_policy.api_cpu',
    ],
  },
  {
    src: 'load balancer web is public\n  listen on 443 with tls for api.example.com\n  send / to api\n\n',
    tint: 'num',
    resources: [
      'aws_security_group.web',
      'aws_vpc_security_group_ingress_rule.web_443',
      'aws_vpc_security_group_egress_rule.web_to_api',
      'aws_vpc_security_group_ingress_rule.api_from_web',
      'aws_lb.web',
      'aws_lb_target_group.web_api',
      'aws_acm_certificate.web_api_example_com',
      'aws_acm_certificate_validation.web_api_example_com',
      'aws_lb_listener.web_443',
    ],
  },
  {
    src: 'api connects to db\n',
    tint: 'verb',
    resources: ['aws_vpc_security_group_ingress_rule.db_from_api', 'aws_iam_role_policy.api_execution'],
  },
  {
    src: 'api reads and writes uploads\n',
    tint: 'kw',
    resources: ['aws_iam_role_policy.api_task'],
  },
]

export const FULL_SOURCE = STEPS.map((s) => s.src).join('')
export const TOTAL_RESOURCES = STEPS.reduce((n, s) => n + s.resources.length, 0)
// Non-blank, non-comment lines of examples/app.clowd.
export const SOURCE_LINES = FULL_SOURCE.split('\n').filter((l) => l.trim()).length
// Measured: `clowd build --target aws` output, and the median of 20 runs of
// `clowd check examples/app.clowd` on an Apple M2.
export const JSON_LINES = 410
export const CHECK_MS = 3.8
