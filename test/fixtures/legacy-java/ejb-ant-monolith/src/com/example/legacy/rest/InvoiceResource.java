package com.example.legacy.rest;

import com.example.legacy.ejb.InvoiceBean;
import com.example.legacy.model.Invoice;

import javax.ejb.EJB;
import javax.ws.rs.Consumes;
import javax.ws.rs.GET;
import javax.ws.rs.POST;
import javax.ws.rs.Path;
import javax.ws.rs.Produces;
import javax.ws.rs.core.MediaType;
import java.math.BigDecimal;
import java.util.List;

@Path("/invoices")
@Produces(MediaType.APPLICATION_JSON)
public class InvoiceResource {

    @EJB
    private InvoiceBean invoiceBean;

    @GET
    public List<Invoice> list() {
        return invoiceBean.listInvoices();
    }

    @POST
    @Consumes(MediaType.APPLICATION_JSON)
    public Invoice create(Invoice in) {
        return invoiceBean.create(in.getCustomer(), in.getAmount() == null
                ? BigDecimal.ZERO
                : in.getAmount());
    }
}
