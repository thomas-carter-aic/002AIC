[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you manage your Serverless application’s security boundaries?

# **SEC 2:** How do you manage your Serverless application’s security boundaries?

Defining and securing your Serverless application’s boundaries ensures isolation for, within, and between [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.").

## Resources

[Understanding AWS IAM Resource Policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_identity-vs-resource.html?ref=wellarchitected)  
 [Understanding how AWS Organization Service Control Policies work](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_manage_policies_about-scps.html?ref=wellarchitected)

## Best Practices:

* **Evaluate and define resource policies**: Resource policies can restrict inbound access to managed services. Consider using resource policies to restrict access to your [component](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") based on the source IP address/range, geolocation, function [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") source, version, alias, queues, etc.

* **Use temporary credentials between resources and [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.")**: Credentials and permissions policies should not be shared between any resources in order to maintain a granular segregation of permissions.

* **Design smaller, single purpose functions**: Grant automated or programmatic access to users or roles with only the minimum privileges needed in order to reduce the risk of unauthorized access. Creating smaller, single purpose functions enables you to keep your permissions aligned to least privileged access, and reduces the risk of compromise since the function will do less.

* **Control network traffic at all layers**: Apply controls for controlling both ingress and egress traffic, including data loss prevention. Functions deployed to virtual private networks must consider network access restrict resource access.

## Improvement Plan

**Evaluate and define resource policies**  

* Understand and determine what resource policies are necessary.
  * Resource policies delineate who has fine-grained access to the resource and what actions they can perform on it. They are evaluated and enforced at AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") level before each AWS service’s own authorization mechanisms are applied, when available.
  * Certain resource policies can also be applied at the AWS Organization level, thus providing guardrail for what actions AWS Accounts within the organization root or organizational unit can do.
  * Review your existing policies and how they’re configured, paying close attention to how permissive individual policies are. Your resource policies should only permit necessary callers.
  

* Implement resource policies to prevent access unauthorized access.
  * For [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), use resource-based policies to provide fine-grained access to layers, and what AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") identities and [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") sources that can invoke your function using a specific version or alias.
  * You can combine resource policies with [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") sources, for example, if being invoked by [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), you can restrict the policy down to the [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") ID, HTTP method, and path of the request.
  * For [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), use resource-based policies to restrict API access to specific [Amazon Virtual Private Cloud](serv.concept.amazonvirtualprivatecloud.en.html "A web service for provisioning a logically isolated section of the AWS cloud where you can launch AWS resources in a virtual network that you define. You control your virtual networking environment, including selection of your own IP address range, creation of subnets, and configuration of route tables and network gateways.") (VPC), [VPC endpoint](serv.concept.vpc-endpoint.en.html "A VPC endpoint enables you to privately connect your VPC to supported AWS services and VPC endpoint services powered by PrivateLink without requiring an internet gateway, NAT device, VPN connection, or AWS Direct Connect connection. Instances in your VPC do not require public IP addresses to communicate with resources in the service. Traffic between your VPC and the other service does not leave the Amazon network."), source IP address/range, AWS Account or AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") users.
  * For [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers."), use resource-based policies to provide fine-grained access to queues to certain AWS services and AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") identities (users, roles, accounts).
  * For [Amazon SNS](serv.concept.sns.en.html "A web service that enables applications, end-users, and devices to instantly send and receive notifications from the cloud."), use resource-based policies to restrict authenticated and non-authenticated actions to topics.
  * For [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), use resource-based policies to provide fine-grained access to tables and [indexes](serv.concept.indexes.en.html "A technology that is designed to make looking up information more efficient.").
  * For EventBridge, use resource-based policies to restrict AWS identities to send and receive [events](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") including to specific [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") buses.  
[AWS Lambda Resource-based Policies](https://docs.aws.amazon.com/lambda/latest/dg/access-control-resource-based.html?ref=wellarchitected)  
[Amazon API Gateway Resource-based Policies](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-resource-policies.html?ref=wellarchitected)  
[Amazon SQS Resource-based Policies](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-basic-examples-of-sqs-policies.html?ref=wellarchitected)  
[Amazon SNS Resource-based Policies](https://docs.aws.amazon.com/sns/latest/dg/sns-using-identity-based-policies.html?ref=wellarchitected)  
[Amazon DynamoDB Resource-based Policies](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/authentication-and-access-control.html?ref=wellarchitected)  
[EventBridge Resource-based Policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/list_amazoneventbridge.html?ref=wellarchitected)  
[Amazon S3 Bucket Policy examples](https://docs.aws.amazon.com/AmazonS3/latest/dev/example-bucket-policies.html?ref=wellarchitected)
  

**Use temporary credentials between resources and components**  

* Use dynamic authentication when accessing [components](serv.concept.component.en.html "The code, configuration and AWS Resources that deliver against a business requirement.") and managed services.
  * Serverless frameworks ensure that AWS resources are provisioned with AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") roles unique per function.
  * AWS SAM automatically creates unique [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") roles for every [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function you create. [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") assumes [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") roles, exposes, and rotates temporary credentials to your functions, enabling your application code to access AWS services without hard-coding credentials.  
[AWS SAM Policy templates](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-policy-templates.html?ref=wellarchitected)
  

**Design smaller, single purpose functions**  

* Create single purpose functions with their own [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") role.
  * Create single purpose [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions to better allow for the creation of AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") roles that are specific to your access requirements.  
[Using an AWS IAM Role to Grant Permissions to Applications](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html?ref=wellarchitected)

    * <u>Examples</u>

      * A large multipurpose function might need access to multiple AWS resources such as [Amazon DynamoDB](serv.concept.dynamodb.en.html "A fully managed NoSQL database service that provides fast and predictable performance with seamless scalability."), [Amazon S3](serv.concept.amazonsimplestorageservice.en.html "Storage for the internet. You can use it to store and retrieve any amount of data at any time, from anywhere on the web."), and [Amazon SQS](serv.concept.amazonsimplequeueservice.en.html "Reliable and scalable hosted queues for storing messages as they travel between computers.") while single purpose invocations will not need access to all of them at the same time
  

* Use least privilege access policies with your users and roles
  * Create AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") policies with the least possible number of privileges granted to a user or role. In the unlikely [event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") of misused credentials, credentials will only be able to perform limited interactions.  
[AWS IAM Best Practices: Least Privilege Principle](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html?ref=wellarchitected)  
[Amazon Partner Blog: Least Privilege Principle](https://www.sumologic.com/blog/pragmatic-aws-principle-of-least-privilege-with-iam/?ref=wellarchitected)
  

* Audit permissions used and remove unnecessary permissions where applicable.
  * With AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS."), use AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") Access Advisor to review when was the last time an AWS service was used from a specific [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") user or role. Using this information, you’re able to remove [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") policies and access from your [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") roles.
  * With [AWS CloudTrail](serv.concept.cloudtrail.en.html "A web service that records AWS API calls for your account and delivers log files to you. The recorded information includes the identity of the API caller, the time of the API call, the source IP address of the API caller, the request parameters, and the response elements returned by the AWS service."), use [AWS CloudTrail](serv.concept.cloudtrail.en.html "A web service that records AWS API calls for your account and delivers log files to you. The recorded information includes the identity of the API caller, the time of the API call, the source IP address of the API caller, the request parameters, and the response elements returned by the AWS service.") [Event](serv.concept.event.en.html "An instance of something happening that is significant to the workload.") History to review individual actions your AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") Role has performed in the past. Using this information, you can detect which permissions were actively used and decide to further remove permissions.  
[AWS IAM Access Advisor](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_access-advisor.html?ref=wellarchitected)  
[Automate analyzing permissions using AWS IAM Access Advisor](https://aws.amazon.com/blogs/security/automate-analyzing-permissions-using-iam-access-advisor/?ref=wellarchitected)  
[Viewing Events with AWS CloudTrail Event History](https://docs.aws.amazon.com/awscloudtrail/latest/userguide/view-cloudtrail-events.html?ref=wellarchitected)
  

**Control network traffic at all layers**  

* Use networking controls to enforce access patterns.
  * Ingress traffic to [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") functions isn’t permitted by default. For egress, you can use security groups to permit your [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function to communicate with other AWS resources; for example, an [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app.") function connecting to an [Amazon ElastiCache](serv.concept.elasticache.en.html "A web service that simplifies deploying, operating, and scaling an in-memory cache in the cloud. The service improves the performance of web applications by providing information retrieval from fast, managed, in-memory caches, instead of relying entirely on slower disk-based databases.") cluster.
  * Routing tables can be used to configure routing to different networking appliances to filter or block access to certain locations, if necessary. [Network ACLs](serv.concept.network-acl.en.html "An optional layer of security that acts as a firewall for controlling traffic in and out of a subnet. You can associate multiple subnets with a single network ACL, but a subnet can be associated with only one network ACL at a time.") can be used to block access to CIDR IP ranges or ports.  
[Working with Network ACLs in Amazon VPC](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-network-acls.html?ref=wellarchitected)
  

* Use detective tools to audit your traffic.
  * The use of [VPC Flow Logs](serv.concept.vpc-flow-logs.en.html "enables you to capture information about the IP traffic going to and from network interfaces in your VPC.") can be used to audit traffic of your functions in order to see where their traffic is being sent to at a granular level.  
[VPC Flow Log record examples](https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-records-examples.html?ref=wellarchitected)  
[Learn from your VPC Flow Logs](https://aws.amazon.com/blogs/aws/learn-from-your-vpc-flow-logs-with-additional-meta-data/?ref=wellarchitected)
  

* Block network access when required
  * Third-party tools enable you to disable outgoing internet traffic with exception to AWS services or allow-listed services.  
[Amazon Partner: PureSec FunctionShield](https://www.puresec.io/function-shield?ref=wellarchitected)
  
[Sign In to the Console](https://console.aws.amazon.com/console/home?nc1=f_ct&src=footer-signin-mobile)

### Learn About AWS

* [What Is AWS?](https://aws.amazon.com/what-is-aws/?nc1=f_cc)
* [What Is Cloud Computing?](https://aws.amazon.com/what-is-cloud-computing/?nc1=f_cc)
* [What Is DevOps?](https://aws.amazon.com/devops/what-is-devops/?nc1=f_cc)
* [What Is a Container?](https://aws.amazon.com/containers/?nc1=f_cc)
* [What Is a Data Lake?](https://aws.amazon.com/big-data/datalakes-and-analytics/what-is-a-data-lake/?nc1=f_cc)
* [AWS Cloud Security](https://aws.amazon.com/security/?nc1=f_cc)
* [What's New](https://aws.amazon.com/new/?nc1=f_cc)
* [Blogs](https://aws.amazon.com/blogs/?nc1=f_cc)
* [Press Releases](https://press.aboutamazon.com/press-releases/aws "Press Releases")

### Resources for AWS

* [Getting Started](https://aws.amazon.com/getting-started/?nc1=f_cc)
* [Training and Certification](https://aws.amazon.com/training/?nc1=f_cc)
* [AWS Solutions Portfolio](https://aws.amazon.com/solutions/?nc1=f_cc)
* [Architecture Center](https://aws.amazon.com/architecture/?nc1=f_cc)
* [Product and Technical FAQs](https://aws.amazon.com/faqs/?nc1=f_dr)
* [Analyst Reports](https://aws.amazon.com/resources/analyst-reports/?nc1=f_cc)
* [AWS Partner Network](https://aws.amazon.com/partners/?nc1=f_dr)

### Developers on AWS

* [Developer Center](https://aws.amazon.com/developer/?nc1=f_dr)
* [SDKs& Tools](https://aws.amazon.com/developer/tools/?nc1=f_dr)
* [.NET on AWS](https://aws.amazon.com/developer/language/net/?nc1=f_dr)
* [Python on AWS](https://aws.amazon.com/developer/language/python/?nc1=f_dr)
* [Java on AWS](https://aws.amazon.com/developer/language/java/?nc1=f_dr)
* [PHP on AWS](https://aws.amazon.com/developer/language/php/?nc1=f_cc)
* [Javascript on AWS](https://aws.amazon.com/developer/language/javascript/?nc1=f_dr)

### Help

* [Contact Us](https://aws.amazon.com/contact-us/?nc1=f_m)
* [AWS Careers](https://aws.amazon.com/careers/?nc1=f_hi)
* [File a Support Ticket](https://console.aws.amazon.com/support/home/?nc1=f_dr)
* [Knowledge Center](https://aws.amazon.com/premiumsupport/knowledge-center/?nc1=f_dr)
* [AWS Support Overview](https://aws.amazon.com/premiumsupport/?nc1=f_dr)
* [Legal](https://aws.amazon.com/legal/?nc1=f_cc)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc1=f_ct&src=default)

Amazon is an Equal Opportunity Employer: *Minority / Women / Disability / Veteran / Gender Identity / Sexual Orientation / Age.*

* Language
* [English](https://wa.aws.amazon.com/index.en.html)
* [日本語](https://wa.aws.amazon.com/index.ja.html)
* [한국어](https://wa.aws.amazon.com/index.ko.html)
* [Français](https://wa.aws.amazon.com/index.fr.html)
* [Português](https://wa.aws.amazon.com/index.pt_BR.html)
* [Deutsch](https://wa.aws.amazon.com/index.de.html)
* [Español](https://wa.aws.amazon.com/index.es.html)
* [Italiano](https://wa.aws.amazon.com/index.it.html)
* [中文 (繁體)](https://wa.aws.amazon.com/index.zh_TW.html)
* [中文 (简体)](https://wa.aws.amazon.com/index.zh_CN.html)

* [Privacy](https://aws.amazon.com/privacy/?nc1=f_pr)
* |
* [Site Terms](https://aws.amazon.com/terms/?nc1=f_pr)
* |
* &copy; 2023, Amazon Web Services, Inc. or its affiliates. All rights reserved.