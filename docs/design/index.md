<!-- -*- fill-column: 100 -*- -->
# Design documents

 - [Principles] sets out the principles for the design
 - [Layering] outlines the API in terms of what each part does.
   - [Assemblages] describes the assemblage layer, which conveys syncs
     to workload clusters
   - [Modules] describes the modules layer, which defines syncs to be
     applied to the fleet of clusters
 - [Bootstrap] describes the mechanisms for bootstrapping the control
   plane and workload clusters
 - [Specialisation] describes the mechanism for specialising
   configurations to clusters
 - [Other designs] discusses other designs relating to GitOps fleet
   management, including ArgoCD ApplicationSets and Rancher Fleet

[Principles]: ./principles.md
[Layering]: ./layering.md
[Assemblages]: ./assemblages.md
[Modules]: ./modules.md
[Bootstrap]: ./bootstrap.md
[Specialisation]: ./specialisation.md
[Other designs]: ./other-designs.md
