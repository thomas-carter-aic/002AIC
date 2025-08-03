[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [DL.LD.8] Generate mock datasets for local development

**Category:** OPTIONAL

Mock datasets are synthetic or modified datasets that developers can use during the development process, eliminating the need to interact with real, sensitive production data. Using mock datasets ensures tests are thorough and realistic, without compromising security.

Use data generating tools to create mock datasets. These tools can range from random data generators to more advanced methods like generative AI. Generative AI can be used to generate synthetic datasets that can be used to test applications and is especially useful for generating data that is not often included in testing datasets, such as defects or edge cases.

If using real-world data is necessary for local development, ensure it is obfuscated. Methods such as masking, encrypting, or tokenizing production datasets can transform real datasets into mock datasets that are safe for local development. It might be useful to store already prepared mock datasets that can be shared between teams or systems to perform testing with. This approach creates a realistic local testing environment without risking developers handling actual production data.

**Related information:**

* [Testing software and systems at Amazon: Developer environment](https://youtu.be/o1sc3cK9bMU?t=1017)

* [Generate test data using an AWS Glue job and Python](https://docs.aws.amazon.com/prescriptive-guidance/latest/patterns/generate-test-data-using-an-aws-glue-job-and-python.html)

* [Foundation Model API Service - Amazon Bedrock](https://aws.amazon.com/bedrock/)

* [What is Generative AI?](https://aws.amazon.com/what-is/generative-ai/)


[Document Conventions](/general/latest/gr/docconventions.html)

\[DL.LD.7] Establish sandbox environments with spend limits

\[DL.LD.9] Share tool configurations

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>