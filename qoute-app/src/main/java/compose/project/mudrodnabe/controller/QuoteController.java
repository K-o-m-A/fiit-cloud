package compose.project.mudrodnabe.controller;

import compose.project.mudrodnabe.domain.QuoteDto;
import compose.project.mudrodnabe.service.QuoteService;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class QuoteController {

    private final QuoteService quoteService;

    public QuoteController(QuoteService quoteService) {
        this.quoteService = quoteService;
    }

    @GetMapping("/quote")
    public QuoteDto index() {
        return quoteService.getQuote();
    }
}
