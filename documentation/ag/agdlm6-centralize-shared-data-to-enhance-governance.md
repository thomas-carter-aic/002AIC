[Documentation](/index.html)[AWS Well-Architected](devops-guidance.html)

# [AG.DLM.6] Centralize shared data to enhance governance

**Category:** FOUNDATIONAL

Practicing DevOps puts an emphasis on teams working collaboratively and continuously exchanging data. Governing this shared data requires proper control, management, and distribution of data to prevent unauthorized access, data breaches, and other security incidents, fostering trust and enhancing the quality and reliability of software delivery.

Use centralized data lakes to provide a single source of truth of data and management within your organization, helping to reduce data silos and inconsistencies. It enables secure and efficient data sharing across teams, enhancing collaboration and overall productivity. Use Role-Based Access Control (RBAC) or Attribute-Based Access Control (ABAC) to limit access to data based on the user context. Implement automated metadata management to better understand the context, source, and lineage of the data, and deploy continuous, automated data quality checks to ensure the accuracy and usability of the data.

When collaboration extends beyond the organization's boundaries, *clean rooms* can be used to maintain data privacy and security. Clean rooms create isolated data processing environments that let multiple parties collaborate and share data in a controlled, privacy-safe manner. With predefined rules that automatically govern the flow and accessibility of data, these clean rooms help ensure data privacy while still allowing for the extraction of valuable insights. This isolation facilitates decision-making and strategic planning, enabling stakeholders to collaborate and share information while protecting user privacy and maintaining compliance with various regulations.

**Related information:**

* [AWS Well-Architected Sustainability Pillar: SUS04-BP06 Use shared file systems or storage to access common data](https://docs.aws.amazon.com/wellarchitected/latest/sustainability-pillar/sus_sus_data_a7.html)

* [Data Collaboration Service - AWS Clean Rooms](https://aws.amazon.com/clean-rooms/)

* [AWS Lake Formation](https://aws.amazon.com/lake-formation/)

* [AWS Data Exchange](https://aws.amazon.com/data-exchange)


[Document Conventions](/general/latest/gr/docconventions.html)

\[AG.DLM.5] Reduce risks and costs with systematic data retention strategies

\[AG.DLM.7] Ensure data safety with automated backup processes

Did this page help you? - Yes

Thanks for letting us know we're doing a good job!

If you've got a moment, please tell us what we did right so we can do more of it.

Did this page help you? - No

Thanks for letting us know this page needs work. We're sorry we let you down.

If you've got a moment, please tell us how we can make the documentation better.</awsdocs-view></awsui-app-layout>