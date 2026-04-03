package main

import (
	"fmt"

	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	autoscalingv2 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/autoscaling/v2"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/networking/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// Application constants.
const (
	AppName       = "remnacore"
	HTTPPort      = 4000
	SpeedTestPort = 4203
	SubProxyPort  = 4100
	PostgresPort  = 5432
	ValkeyPort    = 6379
	NATSPort      = 4222

	DefaultReplicas  = 1
	DefaultNamespace = "remnacore"
	DefaultLogLevel  = "info"
	DefaultImageTag  = "latest"

	ImageRegistry = "ghcr.io/bedolaga-dev/remnacore"
)

// Probe timing constants.
const (
	LivenessInitialDelay  = 10
	LivenessPeriod        = 15
	LivenessTimeout       = 5
	LivenessFailure       = 3
	ReadinessInitialDelay = 5
	ReadinessPeriod       = 10
	ReadinessTimeout      = 5
	ReadinessFailure      = 3
)

// HPA defaults.
const (
	DefaultHPAMinReplicas = 2
	DefaultHPAMaxReplicas = 10
	HPACPUTargetPercent   = 70
	HPAMemTargetPercent   = 80
)

// Security context.
const (
	RunAsUID = 65534
	RunAsGID = 65534
)

// Resource requests and limits.
const (
	RequestCPU    = "100m"
	RequestMemory = "128Mi"
	LimitCPU      = "500m"
	LimitMemory   = "512Mi"
)

// Infrastructure defaults.
const (
	DefaultHealthCheckInterval    = "10s"
	DefaultMaxConcurrentChecks    = "50"
	DefaultPluginDir              = "/app/plugins"
	DefaultPluginMaxPlugins       = "50"
	DefaultBillingTrialDays       = "7"
	DefaultJWTAccessTokenTTL      = "15m"
	DefaultJWTRefreshTokenTTL     = "168h"
	DefaultRemnawaveURL           = "http://remnacore-remnawave:3000"
	JWTKeyMountPath               = "/etc/remnacore/jwt"
)

// standardLabels returns the common Kubernetes labels applied to every resource.
func standardLabels(component string) pulumi.StringMap {
	return pulumi.StringMap{
		"app.kubernetes.io/name":       pulumi.String(AppName),
		"app.kubernetes.io/component":  pulumi.String(component),
		"app.kubernetes.io/managed-by": pulumi.String("pulumi"),
		"app.kubernetes.io/part-of":    pulumi.String(AppName),
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, AppName)

		// --------------- Stack configuration ---------------

		namespace := cfg.Get("namespace")
		if namespace == "" {
			namespace = DefaultNamespace
		}

		replicas := cfg.GetInt("replicas")
		if replicas == 0 {
			replicas = DefaultReplicas
		}

		imageTag := cfg.Get("imageTag")
		if imageTag == "" {
			imageTag = DefaultImageTag
		}

		logLevel := cfg.Get("logLevel")
		if logLevel == "" {
			logLevel = DefaultLogLevel
		}

		enableIngress := cfg.GetBool("enableIngress")
		ingressHost := cfg.Get("ingressHost")

		enableHPA := cfg.GetBool("enableHPA")
		hpaMinReplicas := cfg.GetInt("hpaMinReplicas")
		if hpaMinReplicas == 0 {
			hpaMinReplicas = DefaultHPAMinReplicas
		}
		hpaMaxReplicas := cfg.GetInt("hpaMaxReplicas")
		if hpaMaxReplicas == 0 {
			hpaMaxReplicas = DefaultHPAMaxReplicas
		}

		image := fmt.Sprintf("%s:%s", ImageRegistry, imageTag)

		// --------------- Namespace ---------------

		ns, err := corev1.NewNamespace(ctx, namespace, &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:   pulumi.String(namespace),
				Labels: standardLabels("namespace"),
			},
		})
		if err != nil {
			return err
		}

		// --------------- ConfigMap ---------------
		// Only non-secret env vars. Env var names match the koanf prefix scheme
		// in internal/config/config.go.

		configMap, err := corev1.NewConfigMap(ctx, AppName+"-config", &corev1.ConfigMapArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName + "-config"),
				Namespace: ns.Metadata.Name(),
				Labels:    standardLabels("config"),
			},
			Data: pulumi.StringMap{
				// Application
				"APP_PORT":       pulumi.String(fmt.Sprintf("%d", HTTPPort)),
				"APP_LOG_LEVEL":  pulumi.String(logLevel),
				"APP_LOG_FORMAT": pulumi.String("json"),

				// Valkey (Redis-compatible)
				"VALKEY_URL": pulumi.String(fmt.Sprintf("redis://remnacore-valkey:%d", ValkeyPort)),

				// NATS
				"NATS_URL": pulumi.String(fmt.Sprintf("nats://remnacore-nats:%d", NATSPort)),

				// JWT — keys are mounted as files; paths point to the volume mount.
				"JWT_ACCESS_TOKEN_TTL":  pulumi.String(DefaultJWTAccessTokenTTL),
				"JWT_REFRESH_TOKEN_TTL": pulumi.String(DefaultJWTRefreshTokenTTL),
				"JWT_PRIVATE_KEY_PATH":  pulumi.String(JWTKeyMountPath + "/private.pem"),
				"JWT_PUBLIC_KEY_PATH":   pulumi.String(JWTKeyMountPath + "/public.pem"),

				// Remnawave integration
				"REMNAWAVE_URL": pulumi.String(DefaultRemnawaveURL),

				// Infrastructure
				"INFRA_HEALTH_CHECK_INTERVAL":    pulumi.String(DefaultHealthCheckInterval),
				"INFRA_MAX_CONCURRENT_CHECKS":    pulumi.String(DefaultMaxConcurrentChecks),
				"INFRA_SPEED_TEST_PORT":          pulumi.String(fmt.Sprintf("%d", SpeedTestPort)),
				"INFRA_SUBSCRIPTION_PROXY_PORT":  pulumi.String(fmt.Sprintf("%d", SubProxyPort)),

				// Plugins
				"PLUGIN_DIR":         pulumi.String(DefaultPluginDir),
				"PLUGIN_MAX_PLUGINS": pulumi.String(DefaultPluginMaxPlugins),

				// Billing
				"BILLING_TRIAL_DAYS": pulumi.String(DefaultBillingTrialDays),
			},
		})
		if err != nil {
			return err
		}

		// --------------- Secret ---------------

		// Sensitive values are read with cfg.Get (plain string) then wrapped with
		// pulumi.ToSecret so Pulumi encrypts them in state. Set values via:
		//   pulumi config set --secret remnacore:dbPassword <value>
		dbPassword := cfg.Get("dbPassword")
		if dbPassword == "" {
			dbPassword = "changeme"
		}

		dbUser := cfg.Get("dbUser")
		if dbUser == "" {
			dbUser = "remnacore"
		}

		dbHost := cfg.Get("dbHost")
		if dbHost == "" {
			dbHost = fmt.Sprintf("remnacore-postgres:%d", PostgresPort)
		}

		dbName := cfg.Get("dbName")
		if dbName == "" {
			dbName = "remnacore"
		}

		databaseURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbName)

		jwtPrivateKey := cfg.Get("jwtPrivateKey")
		jwtPublicKey := cfg.Get("jwtPublicKey")

		secretStringData := pulumi.StringMap{
			"DATABASE_URL":           pulumi.ToSecret(pulumi.String(databaseURL)).(pulumi.StringOutput),
			"REMNAWAVE_API_TOKEN":    pulumi.ToSecret(pulumi.String(cfg.Get("remnawaveApiToken"))).(pulumi.StringOutput),
			"REMNAWAVE_WEBHOOK_SECRET": pulumi.ToSecret(pulumi.String(cfg.Get("remnawaveWebhookSecret"))).(pulumi.StringOutput),
			"TELEGRAM_BOT_TOKEN":     pulumi.ToSecret(pulumi.String(cfg.Get("telegramBotToken"))).(pulumi.StringOutput),
			// JWT keys stored in secret for volume mount
			"jwt-private.pem": pulumi.ToSecret(pulumi.String(jwtPrivateKey)).(pulumi.StringOutput),
			"jwt-public.pem":  pulumi.ToSecret(pulumi.String(jwtPublicKey)).(pulumi.StringOutput),
		}

		secret, err := corev1.NewSecret(ctx, AppName+"-secret", &corev1.SecretArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName + "-secret"),
				Namespace: ns.Metadata.Name(),
				Labels:    standardLabels("secret"),
			},
			Type:       pulumi.String("Opaque"),
			StringData: secretStringData,
		})
		if err != nil {
			return err
		}

		// --------------- Deployment ---------------

		appLabels := standardLabels("api")
		replicaCount := pulumi.Int(replicas)

		deployment, err := appsv1.NewDeployment(ctx, AppName, &appsv1.DeploymentArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName),
				Namespace: ns.Metadata.Name(),
				Labels:    appLabels,
			},
			Spec: &appsv1.DeploymentSpecArgs{
				Replicas: replicaCount,
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: pulumi.StringMap{
						"app.kubernetes.io/name":      pulumi.String(AppName),
						"app.kubernetes.io/component": pulumi.String("api"),
					},
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: appLabels,
					},
					Spec: &corev1.PodSpecArgs{
						SecurityContext: &corev1.PodSecurityContextArgs{
							RunAsNonRoot: pulumi.Bool(true),
							RunAsUser:    pulumi.Int(RunAsUID),
							RunAsGroup:   pulumi.Int(RunAsGID),
							FsGroup:      pulumi.Int(RunAsGID),
						},
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String(AppName),
								Image: pulumi.String(image),
								Ports: corev1.ContainerPortArray{
									&corev1.ContainerPortArgs{
										Name:          pulumi.String("http"),
										ContainerPort: pulumi.Int(HTTPPort),
										Protocol:      pulumi.String("TCP"),
									},
									&corev1.ContainerPortArgs{
										Name:          pulumi.String("subproxy"),
										ContainerPort: pulumi.Int(SubProxyPort),
										Protocol:      pulumi.String("TCP"),
									},
									&corev1.ContainerPortArgs{
										Name:          pulumi.String("speedtest"),
										ContainerPort: pulumi.Int(SpeedTestPort),
										Protocol:      pulumi.String("TCP"),
									},
								},
								EnvFrom: corev1.EnvFromSourceArray{
									&corev1.EnvFromSourceArgs{
										ConfigMapRef: &corev1.ConfigMapEnvSourceArgs{
											Name: configMap.Metadata.Name(),
										},
									},
									&corev1.EnvFromSourceArgs{
										SecretRef: &corev1.SecretEnvSourceArgs{
											Name: secret.Metadata.Name(),
										},
									},
								},
								Resources: &corev1.ResourceRequirementsArgs{
									Requests: pulumi.StringMap{
										"cpu":    pulumi.String(RequestCPU),
										"memory": pulumi.String(RequestMemory),
									},
									Limits: pulumi.StringMap{
										"cpu":    pulumi.String(LimitCPU),
										"memory": pulumi.String(LimitMemory),
									},
								},
								LivenessProbe: &corev1.ProbeArgs{
									HttpGet: &corev1.HTTPGetActionArgs{
										Path: pulumi.String("/healthz"),
										Port: pulumi.Int(HTTPPort),
									},
									InitialDelaySeconds: pulumi.Int(LivenessInitialDelay),
									PeriodSeconds:       pulumi.Int(LivenessPeriod),
									TimeoutSeconds:      pulumi.Int(LivenessTimeout),
									FailureThreshold:    pulumi.Int(LivenessFailure),
								},
								ReadinessProbe: &corev1.ProbeArgs{
									HttpGet: &corev1.HTTPGetActionArgs{
										Path: pulumi.String("/readyz"),
										Port: pulumi.Int(HTTPPort),
									},
									InitialDelaySeconds: pulumi.Int(ReadinessInitialDelay),
									PeriodSeconds:       pulumi.Int(ReadinessPeriod),
									TimeoutSeconds:      pulumi.Int(ReadinessTimeout),
									FailureThreshold:    pulumi.Int(ReadinessFailure),
								},
								SecurityContext: &corev1.SecurityContextArgs{
									RunAsNonRoot:             pulumi.Bool(true),
									RunAsUser:                pulumi.Int(RunAsUID),
									ReadOnlyRootFilesystem:   pulumi.Bool(true),
									AllowPrivilegeEscalation: pulumi.Bool(false),
									Capabilities: &corev1.CapabilitiesArgs{
										Drop: pulumi.StringArray{
											pulumi.String("ALL"),
										},
									},
								},
								VolumeMounts: corev1.VolumeMountArray{
									&corev1.VolumeMountArgs{
										Name:      pulumi.String("tmp"),
										MountPath: pulumi.String("/tmp"),
									},
									&corev1.VolumeMountArgs{
										Name:      pulumi.String("jwt-keys"),
										MountPath: pulumi.String(JWTKeyMountPath),
										ReadOnly:  pulumi.Bool(true),
									},
									&corev1.VolumeMountArgs{
										Name:      pulumi.String("plugins"),
										MountPath: pulumi.String(DefaultPluginDir),
									},
								},
							},
						},
						Volumes: corev1.VolumeArray{
							&corev1.VolumeArgs{
								Name: pulumi.String("tmp"),
								EmptyDir: &corev1.EmptyDirVolumeSourceArgs{
									SizeLimit: pulumi.String("64Mi"),
								},
							},
							&corev1.VolumeArgs{
								Name: pulumi.String("jwt-keys"),
								Secret: &corev1.SecretVolumeSourceArgs{
									SecretName: secret.Metadata.Name(),
									Items: corev1.KeyToPathArray{
										&corev1.KeyToPathArgs{
											Key:  pulumi.String("jwt-private.pem"),
											Path: pulumi.String("private.pem"),
										},
										&corev1.KeyToPathArgs{
											Key:  pulumi.String("jwt-public.pem"),
											Path: pulumi.String("public.pem"),
										},
									},
								},
							},
							&corev1.VolumeArgs{
								Name: pulumi.String("plugins"),
								EmptyDir: &corev1.EmptyDirVolumeSourceArgs{
									SizeLimit: pulumi.String("128Mi"),
								},
							},
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// --------------- Services ---------------

		apiService, err := corev1.NewService(ctx, AppName+"-api", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName + "-api"),
				Namespace: ns.Metadata.Name(),
				Labels:    standardLabels("api-service"),
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("ClusterIP"),
				Selector: pulumi.StringMap{
					"app.kubernetes.io/name":      pulumi.String(AppName),
					"app.kubernetes.io/component": pulumi.String("api"),
				},
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Name:       pulumi.String("http"),
						Port:       pulumi.Int(HTTPPort),
						TargetPort: pulumi.Int(HTTPPort),
						Protocol:   pulumi.String("TCP"),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		speedtestService, err := corev1.NewService(ctx, AppName+"-speedtest", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName + "-speedtest"),
				Namespace: ns.Metadata.Name(),
				Labels:    standardLabels("speedtest-service"),
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("ClusterIP"),
				Selector: pulumi.StringMap{
					"app.kubernetes.io/name":      pulumi.String(AppName),
					"app.kubernetes.io/component": pulumi.String("api"),
				},
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Name:       pulumi.String("speedtest"),
						Port:       pulumi.Int(SpeedTestPort),
						TargetPort: pulumi.Int(SpeedTestPort),
						Protocol:   pulumi.String("TCP"),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		subproxyService, err := corev1.NewService(ctx, AppName+"-subproxy", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String(AppName + "-subproxy"),
				Namespace: ns.Metadata.Name(),
				Labels:    standardLabels("subproxy-service"),
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("ClusterIP"),
				Selector: pulumi.StringMap{
					"app.kubernetes.io/name":      pulumi.String(AppName),
					"app.kubernetes.io/component": pulumi.String("api"),
				},
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Name:       pulumi.String("subproxy"),
						Port:       pulumi.Int(SubProxyPort),
						TargetPort: pulumi.Int(SubProxyPort),
						Protocol:   pulumi.String("TCP"),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// --------------- Ingress (optional) ---------------
		//
		// Enabled via config: `pulumi config set remnacore:enableIngress true`
		// Requires: `pulumi config set remnacore:ingressHost api.example.com`

		if enableIngress && ingressHost != "" {
			pathType := pulumi.String("Prefix")
			_, err = networkingv1.NewIngress(ctx, AppName+"-ingress", &networkingv1.IngressArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name:      pulumi.String(AppName + "-ingress"),
					Namespace: ns.Metadata.Name(),
					Labels:    standardLabels("ingress"),
					Annotations: pulumi.StringMap{
						"cert-manager.io/cluster-issuer":                 pulumi.String("letsencrypt-prod"),
						"nginx.ingress.kubernetes.io/ssl-redirect":       pulumi.String("true"),
						"nginx.ingress.kubernetes.io/proxy-body-size":    pulumi.String("10m"),
						"nginx.ingress.kubernetes.io/proxy-read-timeout": pulumi.String("60"),
					},
				},
				Spec: &networkingv1.IngressSpecArgs{
					IngressClassName: pulumi.String("nginx"),
					Tls: networkingv1.IngressTLSArray{
						&networkingv1.IngressTLSArgs{
							Hosts: pulumi.StringArray{
								pulumi.String(ingressHost),
							},
							SecretName: pulumi.String(AppName + "-tls"),
						},
					},
					Rules: networkingv1.IngressRuleArray{
						&networkingv1.IngressRuleArgs{
							Host: pulumi.String(ingressHost),
							Http: &networkingv1.HTTPIngressRuleValueArgs{
								Paths: networkingv1.HTTPIngressPathArray{
									&networkingv1.HTTPIngressPathArgs{
										Path:     pulumi.String("/"),
										PathType: pathType,
										Backend: &networkingv1.IngressBackendArgs{
											Service: &networkingv1.IngressServiceBackendArgs{
												Name: apiService.Metadata.Name().Elem(),
												Port: &networkingv1.ServiceBackendPortArgs{
													Number: pulumi.Int(HTTPPort),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}
		}

		// --------------- HPA (optional) ---------------
		//
		// Enabled via config: `pulumi config set remnacore:enableHPA true`

		if enableHPA {
			_, err = autoscalingv2.NewHorizontalPodAutoscaler(ctx, AppName+"-hpa", &autoscalingv2.HorizontalPodAutoscalerArgs{
				Metadata: &metav1.ObjectMetaArgs{
					Name:      pulumi.String(AppName + "-hpa"),
					Namespace: ns.Metadata.Name(),
					Labels:    standardLabels("hpa"),
				},
				Spec: &autoscalingv2.HorizontalPodAutoscalerSpecArgs{
					ScaleTargetRef: &autoscalingv2.CrossVersionObjectReferenceArgs{
						ApiVersion: pulumi.String("apps/v1"),
						Kind:       pulumi.String("Deployment"),
						Name:       deployment.Metadata.Name().Elem(),
					},
					MinReplicas: pulumi.Int(hpaMinReplicas),
					MaxReplicas: pulumi.Int(hpaMaxReplicas),
					Metrics: autoscalingv2.MetricSpecArray{
						&autoscalingv2.MetricSpecArgs{
							Type: pulumi.String("Resource"),
							Resource: &autoscalingv2.ResourceMetricSourceArgs{
								Name: pulumi.String("cpu"),
								Target: &autoscalingv2.MetricTargetArgs{
									Type:               pulumi.String("Utilization"),
									AverageUtilization: pulumi.Int(HPACPUTargetPercent),
								},
							},
						},
						&autoscalingv2.MetricSpecArgs{
							Type: pulumi.String("Resource"),
							Resource: &autoscalingv2.ResourceMetricSourceArgs{
								Name: pulumi.String("memory"),
								Target: &autoscalingv2.MetricTargetArgs{
									Type:               pulumi.String("Utilization"),
									AverageUtilization: pulumi.Int(HPAMemTargetPercent),
								},
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}
		}

		// --------------- External Dependencies (production notes) ---------------
		//
		// PostgreSQL:
		//   In production, use a managed database service (e.g. AWS RDS, GCP Cloud SQL,
		//   DigitalOcean Managed Databases) or deploy via the Bitnami PostgreSQL Helm chart.
		//   Set the dbHost, dbUser, dbPassword, and dbName config values accordingly.
		//
		// NATS:
		//   In production, deploy via the official NATS Helm chart:
		//     helm repo add nats https://nats-io.github.io/k8s/helm/charts/
		//     helm install remnacore-nats nats/nats
		//   Or use a managed NATS service (e.g. Synadia Cloud).
		//
		// Valkey (Redis-compatible):
		//   In production, use a managed Redis/Valkey service (e.g. AWS ElastiCache,
		//   GCP Memorystore) or deploy via the Bitnami Valkey Helm chart:
		//     helm repo add bitnami https://charts.bitnami.com/bitnami
		//     helm install remnacore-valkey bitnami/valkey

		// --------------- Exports ---------------

		ctx.Export("namespace", ns.Metadata.Name())
		ctx.Export("deploymentName", deployment.Metadata.Name())
		ctx.Export("apiServiceName", apiService.Metadata.Name())
		ctx.Export("speedtestServiceName", speedtestService.Metadata.Name())
		ctx.Export("subproxyServiceName", subproxyService.Metadata.Name())
		ctx.Export("apiEndpoint", pulumi.Sprintf("%s.%s.svc.cluster.local:%d", apiService.Metadata.Name().Elem(), namespace, HTTPPort))
		ctx.Export("speedtestEndpoint", pulumi.Sprintf("%s.%s.svc.cluster.local:%d", speedtestService.Metadata.Name().Elem(), namespace, SpeedTestPort))
		ctx.Export("subproxyEndpoint", pulumi.Sprintf("%s.%s.svc.cluster.local:%d", subproxyService.Metadata.Name().Elem(), namespace, SubProxyPort))

		return nil
	})
}
