[Click here to return to Amazon Web Services homepage](https://aws.amazon.com/?nc2=h_lg)

[Create an AWS Account](https://portal.aws.amazon.com/gp/aws/developer/registration/index.html?nc2=h_ct&src=default)

[![](../../../../Assets/TypeII/png/AWS_WA_TripleHexagon.png)](https://wa.aws.amazon.com/wat.map.en.html)

How do you control access to your Serverless API?

# **SEC 1:** How do you control access to your Serverless API?

Use authentication and authorization mechanisms to prevent unauthorized access, and enforce quotas for public resources.

## Resources

[Controlling and Managing Access to a REST API in Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-control-access-to-api.html?ref=wellarchitected)  
 [Controlling access to GraphQL APIs in AWS AppSync](https://docs.aws.amazon.com/appsync/latest/devguide/security.html?ref=wellarchitected)

## Best Practices:

* **Use appropriate endpoint type and mechanisms to secure access to your API**: API Gateway can have public and private endpoints and the level of mechanisms to provide secure access to each may differ. Consider public endpoints to serve consumers where they may not be part of your network perimeter. Consider private to serve consumers in your network perimeter where you may not want to expose publicly.

* **Scope access based on identity’s metadata**: Authenticated users should be segregated into logical groups, roles, tiers or based on custom authentication token attributes (for example, SAML/JWT claims). Consider using users identity metadata to enable fine-grain control access to resources and actions.

* **Use authentication and authorization mechanisms**: Integrate with an Identity Provider who can validate your API consumers identity (for example, SAML, JWT, etc.) and only authorize access to successfully authenticated consumers instead of API keys. This will help prevent unauthorized access to your [workload](serv.concept.workload.en.html "The set of components that together deliver business value.") from non-authenticated users.

## Improvement Plan

**Use appropriate endpoint type and mechanisms to secure access to your API**  

* Determine your API consumer and choose an API endpoint type.
  * For providing public content, use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") or [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") public endpoints.
  * For providing content with restricted access, use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") with authorization to specific resources, methods, and actions you want to restrict. With [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), restrict access to specific data types, data fields, queries, mutations, or subscriptions.
  * For providing restricted content to a specific [Amazon Virtual Private Cloud](serv.concept.amazonvirtualprivatecloud.en.html "A web service for provisioning a logically isolated section of the AWS cloud where you can launch AWS resources in a virtual network that you define. You control your virtual networking environment, including selection of your own IP address range, creation of subnets, and configuration of route tables and network gateways.") (VPC), [VPC Endpoint](serv.concept.vpc-endpoint.en.html "A VPC endpoint enables you to privately connect your VPC to supported AWS services and VPC endpoint services powered by PrivateLink without requiring an internet gateway, NAT device, VPN connection, or AWS Direct Connect connection. Instances in your VPC do not require public IP addresses to communicate with resources in the service. Traffic between your VPC and the other service does not leave the Amazon network."), a data center, or a specific AWS Account, use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") private endpoints.
  

* Implement security mechanisms appropriate to your API endpoint.
  * With [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") public and private endpoints, you can enable authorization using [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices.") User Pools, Lambda authorizer, AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") and Resource Policies. Use Resource Policies for restricting API consumers to a specific [Amazon Virtual Private Cloud](serv.concept.amazonvirtualprivatecloud.en.html "A web service for provisioning a logically isolated section of the AWS cloud where you can launch AWS resources in a virtual network that you define. You control your virtual networking environment, including selection of your own IP address range, creation of subnets, and configuration of route tables and network gateways.") (VPC), [VPC endpoint](serv.concept.vpc-endpoint.en.html "A VPC endpoint enables you to privately connect your VPC to supported AWS services and VPC endpoint services powered by PrivateLink without requiring an internet gateway, NAT device, VPN connection, or AWS Direct Connect connection. Instances in your VPC do not require public IP addresses to communicate with resources in the service. Traffic between your VPC and the other service does not leave the Amazon network."), source IP address/range, AWS Account or AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") users.
  * With [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") public endpoints, you can enable authorization with [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices.") User Pools, OpenID Connect compliant providers and AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.").
  * For public content and unauthenticated access, both [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") and [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") provide API Key that can be used to track usage. Rate-based rules can be applied using AWS Web Application Firewall to prevent public API consumers from exceeding a configurable threshold of requests.  
[Using API Key for unauthenticated access with AWS AppSync](https://docs.aws.amazon.com/appsync/latest/devguide/security.html?ref=wellarchitected)  
[Using API Key for unauthenticated access with Amazon API Gateway](https://docs.aws.amazon.com/appsync/latest/devguide/security.html?ref=wellarchitected)  
[Using AWS WAF with Amazon API Gateway](https://aws.amazon.com/blogs/compute/amazon-api-gateway-adds-support-for-aws-waf/?ref=wellarchitected)  
[Setting up API Keys with Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/api-gateway-setup-api-key-with-restapi.html?ref=wellarchitected)
  

**Scope access based on identity’s metadata**  

* Review levels of access, identity metadata and segregate consumers into logical groups/tiers.
  * With [JSON](serv.concept.json.en.html "JavaScript Object Notation. A lightweight data interchange format. For information about JSON, see http://www.json.org/.") Web Tokens (JWT) or SAML, ensure you have the right level of information available within token claims to help you develop authorization logic. Use private claims along with a unique namespace for non-public information that can be shared with your application client.
  * Use private claims along with a unique namespace for non-public information that can be shared with your application client.
  * With [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices."), you can use custom attributes or Pre Token Generation Lambda Trigger feature to enrich JWT tokens.  
[Amazon Partner: Understanding JWT Public, Private and Reserved Claims](https://auth0.com/docs/tokens/jwt-claims?ref=wellarchitected)  
[Customizing identity token claims with Amazon Cognito Lambda Triggers](https://docs.aws.amazon.com/cognito/latest/developerguide/user-pool-lambda-pre-token-generation.html?ref=wellarchitected)
  

* Implement authorization logic based on authentication metadata.
  * For authorizing based on custom scopes, use [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices.") to define a resource server with custom scopes. You can also provide differentiated access based on the custom scopes for different application clients.
  * For authorizing based on token claims, use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") Lambda Authorize. With [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), use GraphQL resolvers.
  * For [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features.") Resolvers, AWS Amplify can auto-generate authorization logic via GraphQL Transformers (directives). With GraphQL Transformers, you can generate fine-grained authorization logic by annotating your GraphQL schema to a specific data type, data field and a specific GraphQL operation you want to allow access, including JWT groups or custom claims.  
[Authorizing access based on custom scopes with Amazon API Gateway and Amazon Cognito](https://aws.amazon.com/premiumsupport/knowledge-center/cognito-custom-scopes-api-gateway/?ref=wellarchitected)  
[AWS Lambda Authorizer with Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-use-lambda-authorizer.html?ref=wellarchitected)  
[Community Hero: The Complete Guide to Custom Authorizers with AWS Lambda and Amazon API Gateway](https://www.alexdebrie.com/posts/lambda-custom-authorizers/?ref=wellarchitected)  
[Authorization use cases with AWS AppSync](https://docs.aws.amazon.com/appsync/latest/devguide/security-authorization-use-cases.html?ref=wellarchitected)  
[Autogenerating fine-grained authorization logic with AWS Amplify and AWS AppSync](https://aws-amplify.github.io/docs/cli-toolchain/graphql?sdk=js&ref=wellarchitected)
  

**Use authentication and authorization mechanisms**  

* Evaluate authorization mechanisms.
  * For authorizing access to internal API consumers or other AWS managed services like [AWS Lambda](serv.concept.lambda.en.html "A web service that lets you run code without provisioning or managing servers. You can run code for virtually any type of application or back-end service with zero administration. You can set up your code to automatically trigger from other AWS services or call it directly from any web or mobile app."), you can use AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") built-in authorization.
  * For web applications, [JSON](serv.concept.json.en.html "JavaScript Object Notation. A lightweight data interchange format. For information about JSON, see http://www.json.org/.") Web Tokens are generally accepted for authenticating consumers, and you can use either [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices."), OpenID Connect (OIDC) or Lambda Authorizes.
  * For custom authorization needs, you can use [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale.") Lambda authorizers where you require a higher degree of customization.  
[Community Hero: Picking the correct authorization mechanism in Amazon API Gateway.](https://www.alexdebrie.com/posts/api-gateway-elements/?ref=wellarchitected)
  

* Enforce authorization for non-public resources within your API.
  * Within [Amazon API Gateway](serv.concept.apigateway.en.html "A fully managed service that makes it easy for developers to create, publish, maintain, monitor, and secure APIs at any scale."), you can enable native authorization for users authenticated through [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices.") or AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS."). For authorizing users authenticated by other Identity Providers, use Lambda Authorizers feature.
  * Within [AWS AppSync](serv.concept.awsappsync.en.html "An enterprise level, fully managed GraphQL service with real-time data synchronization and offline programming features."), you can enable native authorization for users authenticated through [Amazon Cognito](serv.concept.cognito.en.html "A web service that makes it easy to save mobile user data, such as app preferences or game state, in the AWS cloud without writing any back-end code or managing any infrastructure. Amazon Cognito offers mobile identity management and data synchronization across devices."), AWS [IAM](serv.concept.iam.en.html "A web service that enables Amazon Web Services customers to manage users and user permissions within AWS.") or any external Identity Provider compliant with OpenID Connect (OIDC).  
[Using Amazon Cognito with Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-integrate-with-cognito.html?ref=wellarchitected)  
[Using AWS IAM with Amazon API Gateway](https://docs.aws.amazon.com/apigateway/latest/developerguide/permissions.html?ref=wellarchitected)  
[Using custom AWS Lambda function authorizers](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-use-lambda-authorizer.html?ref=wellarchitected)  
[Decoding and verifying the signature of a JSON Web Token issued by Amazon Cognito](https://aws.amazon.com/premiumsupport/knowledge-center/decode-verify-cognito-json-token/?ref=wellarchitected)  
[Community Hero: Authorization with custom authorizers, Amazon Cognito, or AWS IAM](https://www.alexdebrie.com/posts/api-gateway-elements/?ref=wellarchitected)  
[Community Hero: The Complete Guide to Custom Authorizers with AWS Lambda and Amazon API Gateway](https://www.alexdebrie.com/posts/lambda-custom-authorizers/?ref=wellarchitected)
  
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