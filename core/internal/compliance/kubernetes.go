package compliance

import (
	"fmt"
	"sort"
)

type k8sObject struct {
	Kind      string
	Name      string
	Namespace string
	Raw       map[string]any
}

func (o k8sObject) id() string { return o.Kind + "/" + o.Name }

var workloadKinds = map[string]bool{"Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true, "Job": true, "Pod": true, "CronJob": true}

func asK8s(m map[string]any) (k8sObject, bool) {
	kind := str(m["kind"])
	if kind == "" || str(m["apiVersion"]) == "" || kind == "Kustomization" {
		return k8sObject{}, false
	}
	meta := mp(m["metadata"])
	return k8sObject{Kind: kind, Name: str(meta["name"]), Namespace: str(meta["namespace"]), Raw: m}, true
}

// podSpec returns the pod spec of a workload.
func podSpec(o k8sObject) map[string]any {
	switch o.Kind {
	case "Pod":
		return mp(o.Raw["spec"])
	case "CronJob":
		return mp(get(o.Raw, "spec", "jobTemplate", "spec", "template", "spec"))
	}
	return mp(get(o.Raw, "spec", "template", "spec"))
}

func checkKubernetes(rep *Report, objs []k8sObject) {
	namespaces := map[string]k8sObject{}
	workloadNS := map[string]bool{}
	defaultDeny := map[string]bool{}

	for _, o := range objs {
		switch {
		case o.Kind == "Namespace":
			namespaces[o.Name] = o
		case o.Kind == "NetworkPolicy":
			spec := mp(o.Raw["spec"])
			if sel, ok := spec["podSelector"]; ok && len(mp(sel)) == 0 {
				for _, t := range list(spec["policyTypes"]) {
					if str(t) == "Ingress" {
						defaultDeny[o.Namespace] = true
					}
				}
			}
		}
		if workloadKinds[o.Kind] {
			rep.Objects++
			from := len(rep.Findings)
			checkWorkload(rep, o)
			applyExemptions(rep, from, parseExemptions(str(get(o.Raw, "metadata", "annotations", "forgelab.io/compliance-exempt"))))
			if o.Kind != "Pod" || o.Namespace != "" {
				workloadNS[o.Namespace] = true
			}
		}
		if o.Kind == "Ingress" {
			rep.Objects++
			from := len(rep.Findings)
			if len(list(get(o.Raw, "spec", "tls"))) == 0 {
				rep.add("K8S-013", o.id(), "no spec.tls; traffic to this ingress is unencrypted")
			}
			applyExemptions(rep, from, parseExemptions(str(get(o.Raw, "metadata", "annotations", "forgelab.io/compliance-exempt"))))
		}
	}

	nsNames := make([]string, 0, len(workloadNS))
	for ns := range workloadNS {
		nsNames = append(nsNames, ns)
	}
	sort.Strings(nsNames)
	for _, ns := range nsNames {
		if ns == "" {
			continue
		}
		obj := "Namespace/" + ns
		rep.Objects++
		from := len(rep.Findings)
		n, declared := namespaces[ns]
		if declared {
			lbl := str(get(n.Raw, "metadata", "labels", "pod-security.kubernetes.io/enforce"))
			if lbl != "baseline" && lbl != "restricted" {
				rep.add("K8S-010", obj, "label pod-security.kubernetes.io/enforce must be baseline or restricted")
			}
		} else {
			rep.add("K8S-010", obj, "namespace is not declared, so no Pod Security Standard is enforced")
		}
		if !defaultDeny[ns] {
			rep.add("K8S-011", obj, "no default-deny ingress NetworkPolicy (empty podSelector, policyTypes Ingress)")
		}
		if declared {
			applyExemptions(rep, from, parseExemptions(str(get(n.Raw, "metadata", "annotations", "forgelab.io/compliance-exempt"))))
		}
	}
}

func checkWorkload(rep *Report, o k8sObject) {
	spec := podSpec(o)
	if spec == nil {
		return
	}
	obj := o.id()
	podSC := mp(spec["securityContext"])

	if isTrue(spec["hostNetwork"]) || isTrue(spec["hostPID"]) || isTrue(spec["hostIPC"]) {
		rep.add("K8S-008", obj, "pod uses a host namespace (hostNetwork, hostPID, or hostIPC)")
	}
	for _, v := range list(spec["volumes"]) {
		if mp(v)["hostPath"] != nil {
			rep.add("K8S-008", obj, "volume %q uses hostPath", str(mp(v)["name"]))
		}
	}
	if seccomp := str(get(podSC, "seccompProfile", "type")); seccomp == "" {
		containersMissing := false
		for _, c := range containers(spec) {
			if str(get(mp(mp(c)["securityContext"]), "seccompProfile", "type")) == "" {
				containersMissing = true
			}
		}
		if containersMissing {
			rep.add("K8S-009", obj, "no seccompProfile (set type RuntimeDefault on the pod or every container)")
		}
	}
	if spec["automountServiceAccountToken"] != false {
		rep.add("K8S-012", obj, "automountServiceAccountToken is not false")
	}

	for _, raw := range containers(spec) {
		c := mp(raw)
		name := fmt.Sprintf("%s container %q", obj, str(c["name"]))
		sc := mp(c["securityContext"])
		if isTrue(sc["privileged"]) {
			rep.add("K8S-001", name, "privileged: true")
		}
		nonRoot := podSC["runAsNonRoot"] == true
		if v, ok := sc["runAsNonRoot"]; ok {
			nonRoot = v == true
		}
		if !nonRoot {
			rep.add("K8S-002", name, "runAsNonRoot is not true")
		}
		if sc["allowPrivilegeEscalation"] != false {
			rep.add("K8S-003", name, "allowPrivilegeEscalation is not false")
		}
		dropAll := false
		for _, cap := range list(get(sc, "capabilities", "drop")) {
			if str(cap) == "ALL" {
				dropAll = true
			}
		}
		if !dropAll {
			rep.add("K8S-004", name, "capabilities.drop does not include ALL")
		}
		if get(mp(c["resources"]), "limits", "memory") == nil {
			rep.add("K8S-005", name, "no memory limit")
		}
		if sc["readOnlyRootFilesystem"] != true {
			rep.add("K8S-006", name, "readOnlyRootFilesystem is not true")
		}
		if !imageTagOK(str(c["image"])) {
			rep.add("K8S-007", name, "image %q is unpinned or uses latest", str(c["image"]))
		}
	}
}

func containers(spec map[string]any) []any {
	return append(list(spec["containers"]), list(spec["initContainers"])...)
}
