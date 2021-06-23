/*
The TencentCloud provider is used to interact with many resources supported by [TencentCloud](https://intl.cloud.tencent.com).
The provider needs to be configured with the proper credentials before it can be used.

Use the navigation on the left to read about the available resources.

-> **Note:** From version 1.9.0 (June 18, 2019), the provider start to support Terraform 0.12.x.

Example Usage

```hcl
# Configure the TencentCloud Provider
provider "tencentcloud" {
  secret_id  = var.secret_id
  secret_key = var.secret_key
  region     = var.region
}

#Configure the TencentCloud Provider with STS
provider "tencentcloud" {
  secret_id  = var.secret_id
  secret_key = var.secret_key
  region     = var.region
  assume_role {
    role_arn         = var.assume_role_arn
    session_name     = var.session_name
    session_duration = var.session_duration
    policy           = var.policy
  }
}
```

Resources List

Provider Data Sources
  tencentcloud_availability_regions
  tencentcloud_availability_zones

Ckafka
  Data Source
    tencentcloud_ckafka_users
    tencentcloud_ckafka_acls
    tencentcloud_ckafka_topics

  Resource
    tencentcloud_ckafka_user
    tencentcloud_ckafka_acl
    tencentcloud_ckafka_topic

Cloud Object Storage(COS)
  Data Source
    tencentcloud_cos_bucket_object
    tencentcloud_cos_buckets

  Resource
    tencentcloud_cos_bucket
    tencentcloud_cos_bucket_object
    tencentcloud_cos_bucket_policy

Cloud Virtual Machine(CVM)
  Data Source
    tencentcloud_image
    tencentcloud_images
    tencentcloud_instance_types
    tencentcloud_instances
    tencentcloud_key_pairs
    tencentcloud_eip
    tencentcloud_eips
    tencentcloud_placement_groups
    tencentcloud_reserved_instance_configs
    tencentcloud_reserved_instances

  Resource
    tencentcloud_instance
    tencentcloud_eip
    tencentcloud_eip_association
    tencentcloud_key_pair
    tencentcloud_placement_group
    tencentcloud_reserved_instance
    tencentcloud_image

Elasticsearch
  Data Source
    tencentcloud_elasticsearch_instances

  Resource
    tencentcloud_elasticsearch_instance
*/
package xac

import (
	"net/url"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

const (
	PROVIDER_SECRET_ID                    = "TENCENTCLOUD_SECRET_ID"
	PROVIDER_SECRET_KEY                   = "TENCENTCLOUD_SECRET_KEY"
	PROVIDER_SECURITY_TOKEN               = "TENCENTCLOUD_SECURITY_TOKEN"
	PROVIDER_REGION                       = "TENCENTCLOUD_REGION"
	PROVIDER_PROTOCOL                     = "TENCENTCLOUD_PROTOCOL"
	PROVIDER_DOMAIN                       = "TENCENTCLOUD_DOMAIN"
	PROVIDER_ASSUME_ROLE_ARN              = "TENCENTCLOUD_ASSUME_ROLE_ARN"
	PROVIDER_ASSUME_ROLE_SESSION_NAME     = "TENCENTCLOUD_ASSUME_ROLE_SESSION_NAME"
	PROVIDER_ASSUME_ROLE_SESSION_DURATION = "TENCENTCLOUD_ASSUME_ROLE_SESSION_DURATION"
)

type TencentCloudClient struct {
	apiV3Conn *connectivity.TencentCloudClient
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc(PROVIDER_SECRET_ID, nil),
				Description: "This is the TencentCloud access key. It must be provided, but it can also be sourced from the `TENCENTCLOUD_SECRET_ID` environment variable.",
			},
			"secret_key": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc(PROVIDER_SECRET_KEY, nil),
				Description: "This is the TencentCloud secret key. It must be provided, but it can also be sourced from the `TENCENTCLOUD_SECRET_KEY` environment variable.",
				Sensitive:   true,
			},
			"security_token": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(PROVIDER_SECURITY_TOKEN, nil),
				Description: "TencentCloud Security Token of temporary access credentials. It can be sourced from the `TENCENTCLOUD_SECURITY_TOKEN` environment variable. Notice: for supported products, please refer to: [temporary key supported products](https://intl.cloud.tencent.com/document/product/598/10588).",
				Sensitive:   true,
			},
			"region": {
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc(PROVIDER_REGION, nil),
				Description:  "This is the TencentCloud region. It must be provided, but it can also be sourced from the `TENCENTCLOUD_REGION` environment variables. The default input value is ap-guangzhou.",
				InputDefault: "ap-guangzhou",
			},
			"protocol": {
				Type:         schema.TypeString,
				Optional:     true,
				DefaultFunc:  schema.EnvDefaultFunc(PROVIDER_PROTOCOL, "HTTPS"),
				ValidateFunc: validateAllowedStringValue([]string{"HTTP", "HTTPS"}),
				Description:  "The protocol of the API request. Valid values: `HTTP` and `HTTPS`. Default is `HTTPS`.",
			},
			"domain": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc(PROVIDER_DOMAIN, nil),
				Description: "The root domain of the API request, Default is `tencentcloudapi.com`.",
			},
			"assume_role": {
				Type:        schema.TypeSet,
				Optional:    true,
				MaxItems:    1,
				Description: "The `assume_role` block. If provided, terraform will attempt to assume this role using the supplied credentials.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"role_arn": {
							Type:        schema.TypeString,
							Required:    true,
							DefaultFunc: schema.EnvDefaultFunc(PROVIDER_ASSUME_ROLE_ARN, nil),
							Description: "The ARN of the role to assume. It can be sourced from the `TENCENTCLOUD_ASSUME_ROLE_ARN`.",
						},
						"session_name": {
							Type:        schema.TypeString,
							Required:    true,
							DefaultFunc: schema.EnvDefaultFunc(PROVIDER_ASSUME_ROLE_SESSION_NAME, nil),
							Description: "The session name to use when making the AssumeRole call. It can be sourced from the `TENCENTCLOUD_ASSUME_ROLE_SESSION_NAME`.",
						},
						"session_duration": {
							Type:         schema.TypeInt,
							Required:     true,
							InputDefault: "7200",
							ValidateFunc: validateIntegerInRange(0, 43200),
							Description:  "The duration of the session when making the AssumeRole call. Its value ranges from 0 to 43200(seconds), and default is 7200 seconds. It can be sourced from the `TENCENTCLOUD_ASSUME_ROLE_SESSION_DURATION`.",
						},
						"policy": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "A more restrictive policy when making the AssumeRole call. Its content must not contains `principal` elements. Notice: more syntax references, please refer to: [policies syntax logic](https://intl.cloud.tencent.com/document/product/598/10603).",
						},
					},
				},
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"tencentcloud_cos_bucket_object": &schema.Resource{},
			"tencentcloud_cos_buckets":       &schema.Resource{},
			"tencentcloud_audit_cos_regions": &schema.Resource{},
		},

		ResourcesMap: map[string]*schema.Resource{
			"tencentcloud_cos_bucket":        &schema.Resource{},
			"tencentcloud_cos_bucket_object": &schema.Resource{},
			"tencentcloud_cos_bucket_policy": &schema.Resource{},
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	secretId := d.Get("secret_id").(string)
	secretKey := d.Get("secret_key").(string)
	securityToken := d.Get("security_token").(string)
	region := d.Get("region").(string)
	protocol := d.Get("protocol").(string)
	domain := d.Get("domain").(string)

	// standard client
	var tcClient TencentCloudClient
	tcClient.apiV3Conn = &connectivity.TencentCloudClient{
		Credential: common.NewTokenCredential(
			secretId,
			secretKey,
			securityToken,
		),
		Region:   region,
		Protocol: protocol,
		Domain:   domain,
	}

	// assume role client
	assumeRoleList := d.Get("assume_role").(*schema.Set).List()
	if len(assumeRoleList) == 1 {
		assumeRole := assumeRoleList[0].(map[string]interface{})
		assumeRoleArn := assumeRole["role_arn"].(string)
		assumeRoleSessionName := assumeRole["session_name"].(string)
		assumeRoleSessionDuration := assumeRole["session_duration"].(int)
		assumeRolePolicy := assumeRole["policy"].(string)
		if assumeRoleSessionDuration == 0 {
			var err error
			if duration := os.Getenv(PROVIDER_ASSUME_ROLE_SESSION_DURATION); duration != "" {
				assumeRoleSessionDuration, err = strconv.Atoi(duration)
				if err != nil {
					return nil, err
				}
				if assumeRoleSessionDuration == 0 {
					assumeRoleSessionDuration = 7200
				}
			}
		}
		// applying STS credentials
		request := sts.NewAssumeRoleRequest()
		request.RoleArn = &assumeRoleArn
		request.RoleSessionName = &assumeRoleSessionName
		var ds uint64 = uint64(assumeRoleSessionDuration)
		request.DurationSeconds = &ds
		policy := url.QueryEscape(assumeRolePolicy)
		if assumeRolePolicy != "" {
			request.Policy = &policy
		}
		ratelimit.Check(request.GetAction())
		response, err := tcClient.apiV3Conn.UseStsClient().AssumeRole(request)
		if err != nil {
			return nil, err
		}
		// using STS credentials
		tcClient.apiV3Conn.Credential = common.NewTokenCredential(
			*response.Response.Credentials.TmpSecretId,
			*response.Response.Credentials.TmpSecretKey,
			*response.Response.Credentials.Token,
		)
	}

	return &tcClient, nil
}
